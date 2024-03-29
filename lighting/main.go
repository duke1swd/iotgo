/*
 * Lighting daemon.  See README
 */

package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/eclipse/paho.mqtt.golang"
)

const defaultLogDirectory = "/var/log"
const defaultLogFileName = "HomeLighting.log"
const defaultMqttBroker = "tcp://localhost:1883"
const defaultStateMachineTicker = 10 // number of seconds between pokes of the state machine.
const defaultSeasonStartMonth = 11
const defaultSeasonStartDay = 1
const defaultSeasonEndMonth = 1
const defaultSeasonEndDay = 6

type updateType struct {
	update string
	region string
	value1 string
	value2 string
}

type deviceType struct {
	region string
	outlet string
	button string
	active bool // used only in adding dropping devices.
}

type publishType struct {
	topic   string
	payload string
}

/*
 * Things that can go in a region map

 key		value
 ---		-----
 control	auto/manual-i/manual-o
 state		on/off
 command	on/off
 season/start	mm:dd or "light" or nothing
 season/end	mm:dd
 window-start
 window-end


*/

var (
	client            mqtt.Client
	logDirectory      string
	mqttBroker        string
	fullLogFileName   string
	updateChan        chan updateType
	deviceBackChan    chan string
	publishChan       chan publishType
	seasonStart       time.Duration
	seasonEnd         time.Duration
	globalEnable      bool
	verboseLog        bool
	regionMap         map[string]map[string]string // map region name to a region.  A region is a set of string-key/string-value pairs.
	deviceMap         map[string]deviceType        // map a device name to its region
	lightLevel        int
	debug             bool
	lastPublish       time.Time
	stateMachineDefer time.Duration
	loc               *time.Location
)

func init() {
	logDirectory = os.Getenv("LOGDIR")
	if len(logDirectory) < 1 {
		logDirectory = defaultLogDirectory
	}

	logFileName := os.Getenv("LOGFILENAME")
	if len(logFileName) < 1 {
		logFileName = defaultLogFileName
	}

	fullLogFileName = filepath.Join(logDirectory, logFileName)

	mqttBroker = os.Getenv("MQTTBROKER")
	if len(mqttBroker) < 1 {
		mqttBroker = defaultMqttBroker
	}

	_, verboseLog = os.LookupEnv("VERBOSE_LOG")

	flag.BoolVar(&debug, "D", false, "debugging")
	flag.Parse()
	if debug {
		verboseLog = true
	}

	loc, _ = time.LoadLocation("Local")
	lastPublish = time.Now()

	updateChan = make(chan updateType)
	deviceBackChan = make(chan string, 100)
	publishChan = make(chan publishType, 100)
	regionMap = make(map[string]map[string]string)
	deviceMap = make(map[string]deviceType)
	lightLevel = 0
	globalEnable = false
	stateMachineDefer = time.Duration(2) * time.Second
}

// parse "mm/dd" spec
// returns duration after start of year
func parsemmdd(mmdd string, defaultMonth, defaultDay int) (time.Duration, bool) {
	now := time.Now()
	yearBase := time.Date(now.Year(), time.Month(1), 1, 0, 0, 0, 0, loc)
	defaultReturnValue := time.Date(now.Year(), time.Month(defaultMonth), defaultDay, 0, 0, 0, 0, loc).Sub(yearBase)
	if debug {
		fmt.Println("\t\t\tParsing: " + mmdd)
	}
	mc := strings.Split(mmdd, "/")
	if len(mc) != 2 {
		return defaultReturnValue, false
	}

	month, err := strconv.ParseInt(mc[0], 10, 32)
	if err != nil {
		return defaultReturnValue, false
	}

	day, err := strconv.ParseInt(mc[1], 10, 32)
	if err != nil {
		return defaultReturnValue, false
	}

	if month < 1 || month > 12 || day < 1 || day > 31 {
		return defaultReturnValue, false
	}

	return time.Date(now.Year(), time.Month(month), int(day), 0, 0, 0, 0, loc).Sub(yearBase), true
}

// All mqtt messages about lighting are handled here
var lightingHandler mqtt.MessageHandler = func(client mqtt.Client, msg mqtt.Message) {
	payload := string(msg.Payload())
	// ignore messages that are just erasing state
	if payload == "" {
		return
	}
	topic := string(msg.Topic())
	topicComponents := strings.Split(topic, "/")

	if debug {
		fmt.Printf("lighting message: %s %s\n", topic, payload)
	}

	if len(topicComponents) < 2 {
		return
	}

	switch topicComponents[1] {
	case "enable":
		switch payload {
		case "true":
			globalEnable = true
			logMessage("Lighting control enabled")
		case "false":
			globalEnable = false
			logMessage("Lighting control disabled")
		}
	default:
		var update updateType
		update.update = "region"
		update.region = topicComponents[1]
		update.value1 = topicComponents[2]
		update.value2 = payload
		updateChan <- update
	}
}

// All mqtt messages about light level are handled here
var lightHandler mqtt.MessageHandler = func(client mqtt.Client, msg mqtt.Message) {
	payload := string(msg.Payload())

	if debug {
		fmt.Printf("light message: %s\n", payload)
	}

	var update updateType
	update.update = "light"
	update.value1 = payload
	updateChan <- update
}

// device messages come here
var deviceHandler mqtt.MessageHandler = func(client mqtt.Client, msg mqtt.Message) {
	var update updateType
	payload := string(msg.Payload())
	topic := string(msg.Topic())

	// surpress '$' topics, as they are uninteresting in this context
	if strings.Index(topic, "$") >= 0 {
		return
	}

	if debug {
		fmt.Printf("device message: %s %s\n", topic, payload)
	}

	topicComponents := strings.Split(topic, "/")
	if len(topicComponents) < 4 {
		return
	}

	device := topicComponents[1]
	update.value1 = device
	update.value2 = payload

	if topicComponents[2] == "outlet" && topicComponents[3] == "on" && len(topicComponents) == 4 {
		update.update = "outlet"
		updateChan <- update
		return
	}

	if topicComponents[2] == "button" && topicComponents[3] == "button" && len(topicComponents) == 4 {
		update.update = "button"
		updateChan <- update
		return
	}
	if debug {
		fmt.Println("\tmessage discarded")
	}
}

/*
 * This routine processes a new device list for a region.
 * Replaces old device list.
 *
 * Runs in the updater gothread, so is OK to manipulate deviceMap
 */
func updateDevices(region, devices string) {
	// mark all devices inactive
	for name, device := range deviceMap {
		device.active = false
		deviceMap[name] = device
	}

	// for every device mentioned, move to this region
	// and mark it active
	for _, deviceName := range strings.Split(devices, ",") {
		if !validDevice(deviceName) {
			logMessage(fmt.Sprintf("Invalid device name \"%s\" rejected", deviceName))
			continue
		}

		// Get the device, if any, and initialize it to a good state
		device, ok := deviceMap[deviceName]
		if !ok {
			// New device
			device.button = "false"
			device.outlet = "false"
			device.region = region
			deviceBackChan <- deviceName // tell main thread to subscribe
			logMessage(fmt.Sprintf("New device %s in region %s", deviceName, region))
		}
		device.active = true
		if device.region != region {
			logMessage(fmt.Sprintf("Device %s moved from region %s to %s", deviceName, device.region, region))
		}
		device.region = region
		deviceMap[deviceName] = device
	}

	// Now, for every device in this region that is inactive, drop it
	for deviceName, device := range deviceMap {
		if device.region == region && !device.active {
			// we should stop subscribing, but that is too much work.
			delete(deviceMap, deviceName)
			logMessage(fmt.Sprintf("Device %s in region %s dropped", deviceName, device.region))
		}
	}
}

func dropRegion(regionName string) {
	// drop all devices in this region
	logMessage("Dropping region " + regionName)
	for deviceName, device := range deviceMap {
		if device.region == regionName {
			delete(deviceMap, deviceName)
			logMessage("Dropping device " + deviceName)
		}
	}

	// erase all region messages from mqtt
	for topic, _ := range regionMap[regionName] {
		var p publishType
		p.topic = "lighting/" + regionName + "/" + topic
		p.payload = ""
		publishChan <- p
		if verboseLog {
			logMessage("Erasing topic " + p.topic)
		}
	}

	delete(regionMap, regionName)
	logMessage("Region " + regionName + " dropped")
}

func publishControl(name string, control string) {
	var p publishType
	p.topic = fmt.Sprintf("lighting/%s/control", name)
	p.payload = control
	publishChan <- p
}

/*
 * All action requests come here and are serialized that way
 */
func updater() {
	if debug {
		fmt.Println("Updater running")
	}

	tickerDuration := time.Duration(defaultStateMachineTicker) * time.Second
	ticker := time.NewTicker(tickerDuration)
	defer ticker.Stop()

	for {
		select {
		case update := <-updateChan:
			if debug {
				fmt.Printf("Update recieved: %s\n", update.update)
			}
			switch update.update {
			case "region":
				// First, put this data into the region map
				region, ok := regionMap[update.region]
				if !ok {
					region = make(map[string]string)
					region["control"] = "auto"
					if update.value1 != "control" {
						publishControl(update.region, "auto")
					}
				}
				region[update.value1] = update.value2
				regionMap[update.region] = region

				// Some region messages require more processing
				switch update.value1 {
				case "devices":
					updateDevices(update.region, update.value2)
				case "drop":
					dropRegion(update.region)
				}

			case "light":
				// This is done here, rather than in the message handler
				// so that light messages will run the state machine.
				// Threading model doesn't care
				l, err := strconv.ParseInt(update.value1, 10, 32)
				if err == nil {
					lightLevel = int(l)
				}

			case "outlet":
				device, ok := deviceMap[update.value1]
				if ok {
					// Try to figure out if the outlet state was changed by an external entity.
					// If so, count this as a button press.
					// Any change of the outlet to a state different than the region state is presumed external.

					// first check if the state has changed
					if debug {
						fmt.Printf("\t\tGot Outlet Update %s %s\n", update.value1, update.value2)
					}
					if device.outlet != update.value2 {
						device.outlet = update.value2
						if debug {
							fmt.Printf("\t\t\tChanged\n")
						}

						// did we just change to a state that is not the region state?
						region, ok := regionMap[device.region]
						if ok && ((device.outlet == "true" && region["state"] == "off") ||
							(device.outlet == "false" && region["state"] == "on")) {
							device.button = "true"
							if debug {
								fmt.Printf("\t\tXXX Set device %s outlet set to %s trigger inferred button\n",
									update.value1, device.outlet)
							}
						}
						deviceMap[update.value1] = device
					}
				}
			case "button":
				device, ok := deviceMap[update.value1]
				if ok {
					device.button = update.value2
					deviceMap[update.value1] = device
					if debug {
						fmt.Printf("\tSet device %s button to %s\n", update.value1, device.button)
					}
				}
			}
			//ticker.Reset(tickerDuration)	// the docs say this method exists, but it doesn't?
		case _ = <-ticker.C:
			if debug {
				fmt.Println("Updater timeout")
			}
		}
		stateMachine(client)
	}
}

// parse "hh:mm" spec
// returns duration after midnight.
func parsehhmm(hhmm string, defaultHour int) time.Duration {
	defaultReturnValue := time.Duration(defaultHour) * time.Hour
	if debug {
		fmt.Println("\t\t\tParsing: " + hhmm)
	}
	hc := strings.Split(hhmm, ":")
	if len(hc) != 2 {
		return defaultReturnValue
	}

	hour, err := strconv.ParseInt(hc[0], 10, 32)
	if err != nil {
		return defaultReturnValue
	}

	min, err := strconv.ParseInt(hc[1], 10, 32)
	if err != nil {
		return defaultReturnValue
	}

	if hour < 0 || hour > 23 || min < 0 || min > 59 {
		return defaultReturnValue
	}

	return time.Duration(hour)*time.Hour + time.Duration(min)*time.Minute
}

// takes a specification in the form of "hh:mm" and decides when that is
func hhmmWindow(now time.Time, spec string, defaultHour int) time.Time {
	when := parsehhmm(spec, defaultHour)

	dayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)

	return dayStart.Add(when)
}

func setRegionState(regionName string, newState bool) {

	region := regionMap[regionName]
	// Now, see if this matches the public state
	state, ok := region["state"]
	if !ok || (newState && state != "on") || (!newState && state == "on") {
		state = "off"
		if newState {
			state = "on"
		}
		// don't need to update the region map, as we'll recieve the message we are about to publish

		var p publishType
		p.topic = fmt.Sprintf("lighting/%s/state", regionName)
		p.payload = state
		publishChan <- p
		logMessage(fmt.Sprintf("Set region %s to %s", regionName, state))
	}

	// for each device, check whether its state matches the desired state
	// and set the device if necessary
	for deviceName, device := range deviceMap {
		if device.region == regionName {
			var p publishType
			p.topic = fmt.Sprintf("devices/%s/outlet/on/set", deviceName)
			if device.outlet == "true" && !newState {
				p.payload = "false"
				device.outlet = p.payload
				deviceMap[deviceName] = device
				publishChan <- p
				if verboseLog {
					logMessage(fmt.Sprintf("device %s in region %s set to off", deviceName, regionName))
				}
			}
			if device.outlet == "false" && newState {
				p.payload = "true"
				device.outlet = p.payload
				deviceMap[deviceName] = device
				publishChan <- p
				if verboseLog {
					logMessage(fmt.Sprintf("device %s in region %s set to on", deviceName, regionName))
				}
			}
		}
	}
}

// turn off all regions.  Either we are out of season or system is disabled
func allOff() {
	for regionName, _ := range regionMap {
		setRegionState(regionName, false)
	}
}

/*
   This routine gets called from time to time when the state of
   the world may have changed.  Its job is to evaluate the world,
   turning lights on and off when required.  It also acknowledges
   the button pushes on the devices.

   This routine is called from the updater go routine.  It is
   safe for it to access the deviceMap and the regionMap.
*/
func stateMachine(client mqtt.Client) {
	now := time.Now()

	if debug {
		fmt.Println("State Machine Running")
	}
	// First, acknowledge button pushes
	buttonPress := false
	for deviceName, device := range deviceMap {
		if device.button == "true" {
			if verboseLog {
				logMessage(fmt.Sprintf("button on device %s pushed", deviceName))
			}
			if debug {
				fmt.Printf("button on device %s pushed\n", deviceName)
			}
			var p publishType
			p.topic = fmt.Sprintf("devices/%s/button/button/set", deviceName)
			p.payload = "false"
			publishChan <- p
			buttonPress = true
		}
	}

	// dont' delay if we've just seen a command
	for _, region := range regionMap {
		if _, ok := region["command"]; ok {
			buttonPress = true
		}
	}

	// If we've just published some stuff then don't run the state machine
	if !buttonPress && now.Sub(lastPublish) < stateMachineDefer {
		return
	}

	// Is lighting control enabled?
	if !globalEnable {
		allOff()
		return
	}

	if debug {
		fmt.Println("\tEnabled")
		fmt.Println("\tLight level is", lightLevel)
	}

	// For each region
	for regionName, region := range regionMap {

		// Are we in the season?
		inSeason := true
		seasonStartString, ok1 := region["season/start"]
		seasonEndString, ok2 := region["season/end"]
		if ok1 && ok2 {
			inSeason = false

			seasonStart, ok1 := parsemmdd(seasonStartString, defaultSeasonStartMonth, defaultSeasonStartDay)
			seasonEnd, ok2 := parsemmdd(seasonEndString, defaultSeasonEndMonth, defaultSeasonEndDay)
			if ok1 && ok2 {
				start := time.Date(now.Year(), time.Month(1), 1, 0, 0, 0, 0, loc).Add(seasonStart)
				end := time.Date(now.Year(), time.Month(1), 1, 0, 0, 0, 0, loc).Add(seasonEnd).Add(time.Duration(24+9) * time.Hour)
				if start.Before(end) {
					if now.After(start) && now.Before(end) {
						inSeason = true
					}
				} else if now.After(start) || now.Before(end) {
					inSeason = true
				}

				if debug {
					fmt.Println("\tIn season: ", inSeason)
				}
			}
		}

		// Are we in the window when the lights should be on?
		if debug {
			fmt.Println("\tRegion: ", regionName)
		}

		startString, ok := region["window-start"]
		if !ok {
			startString = "light"
		}

		start := hhmmWindow(now, startString, 15)
		end := hhmmWindow(now, region["window-end"], 23)

		inWindow := false
		if start.Before(end) {
			if now.After(start) && now.Before(end) {
				inWindow = true
			}
		} else if now.After(start) || now.Before(end) {
			inWindow = true
		}

		// if we are nominally in the window, but it is not yet dark, ...
		if startString == "light" && inWindow && lightLevel >= 4 {
			inWindow = false
		}

		if debug {
			fmt.Printf("\t\tIn window at light level %d: %v\n", lightLevel, inWindow)
		}

		// handle button pushes and automatic vs manual states
		for deviceName, device := range deviceMap {
			if device.region == regionName && device.button == "true" {
				if debug {
					fmt.Printf("\t\tXXX processing button press.  Region control is %s\n", region["control"])
				}
				device.button = "false"
				deviceMap[deviceName] = device
				switch region["control"] {
				case "manual-i":
					region["control"] = "auto"
				case "manual-o":
					region["control"] = "auto"
				case "auto":
					if inWindow {
						region["control"] = "manual-i"
					} else {
						region["control"] = "manual-o"
					}
				}
				if verboseLog {
					logMessage(fmt.Sprintf("region %s control set to %s by button", regionName, region["control"]))
				}
				regionMap[regionName] = region

				if debug {
					fmt.Printf("\t\t%s[\"control\"] set to %s\n", regionName, region["control"])
				}
				publishControl(regionName, region["control"])
			}
		}

		// handle external commands
		if cmd, ok := region["command"]; ok {
			if verboseLog {
				logMessage(fmt.Sprintf("command %s on region %s received", cmd, regionName))
			}
			switch cmd {
			case "on":
				if !inWindow {
					region["control"] = "manual-o"
				} else {
					region["control"] = "auto"
				}
			case "off":
				if inWindow {
					region["control"] = "manual-i"
				} else {
					region["control"] = "auto"
				}
			case "toggle":
				if inWindow {
					region["control"] = "manual-i"
				} else {
					region["control"] = "manual-o"
				}
			}
			delete(region, "command")

			if verboseLog {
				logMessage(fmt.Sprintf("region %s control set to %s", regionName, region["control"]))
			}

			publishControl(regionName, region["control"])

			var p publishType
			p.topic = fmt.Sprintf("lighting/%s/command", regionName)
			p.payload = ""
			publishChan <- p
		}

		// If manual control has expired, return to automatic control
		if inWindow && region["control"] == "manual-o" {
			region["control"] = "auto"
			publishControl(regionName, "auto")
			if verboseLog {
				logMessage(fmt.Sprintf("region %s control set to auto by window change", regionName))
			}
		}

		if !inWindow && region["control"] == "manual-i" {
			region["control"] = "auto"
			publishControl(regionName, "auto")
			if verboseLog {
				logMessage(fmt.Sprintf("region %s control set to auto by window change", regionName))
			}
		}

		// Calculate whether the lights in this region should be on.
		shouldBeOn := inWindow
		if region["control"] == "manual-i" || region["control"] == "manual-o" {
			shouldBeOn = !shouldBeOn
		}

		if debug {
			fmt.Println("\t\tlights should be on:", shouldBeOn)
		}

		setRegionState(regionName, shouldBeOn)
	}
}

func main() {

	go updater()

	//mqtt.DEBUG = log.New(os.Stdout, "", 0)
	logMessage("Lighting Daemon started")
	logMessage("mqtt broker = " + mqttBroker)
	mqtt.ERROR = log.New(os.Stdout, "", 0)
	opts := mqtt.NewClientOptions().AddBroker(mqttBroker).SetClientID("lighting-daemon")
	opts.SetKeepAlive(60 * time.Second)
	opts.SetPingTimeout(1 * time.Second)

	client = mqtt.NewClient(opts)
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		panic(token.Error())
	}

	if token := client.Subscribe("lighting/#", 0, lightingHandler); token.Wait() && token.Error() != nil {
		fmt.Println(token.Error())
		os.Exit(1)
	}

	if token := client.Subscribe("environment/outdoor-light", 0, lightHandler); token.Wait() && token.Error() != nil {
		fmt.Println(token.Error())
		os.Exit(1)
	}

	// sleep forever, processing requests for mqtt work
	for {
		select {
		case deviceName := <-deviceBackChan:
			sub := "devices/" + deviceName + "/#"
			if token := client.Subscribe(sub, 0, deviceHandler); token.Wait() && token.Error() != nil {
				logMessage(fmt.Sprintf("Failed to subscribe to %s.  Err=%v", sub, token.Error()))
			} else if verboseLog {
				logMessage(fmt.Sprintf("Subscribed to %s", sub))
			}
		case pubRequest := <-publishChan:
			if debug {
				if pubRequest.payload == "" {
					fmt.Println("Erasing", pubRequest.topic)
				} else {
					fmt.Println("Publishing", pubRequest.topic, ": ", pubRequest.payload)
				}
			}
			lastPublish = time.Now()
			client.Publish(pubRequest.topic, 0, true, pubRequest.payload)
		}
	}
}

func logMessage(m string) {
	formattedMsg := time.Now().Format("Mon Jan 2 15:04:05 2006") + "  " + m + "\n"

	// if debugging just send to stdout
	if debug {
		fmt.Print(formattedMsg)
		return
	}

	f, err := os.OpenFile(fullLogFileName, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		log.Printf("Logger: Cannot open for writing log file %s. err = %v", fullLogFileName, err)
		return
	}
	defer f.Close()

	_, err = f.WriteString(formattedMsg)
	if err != nil {
		log.Printf("Logger: Error writing to file %s.  err = %v\n", fullLogFileName, err)
		return
	}
}
