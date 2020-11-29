/*
 * Christmas daemon.  See README
 */

package main

import (
	"context"
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
const defaultLogFileName = "HomeChristmas"
const defaultMqttBroker = "tcp://localhost:1883"

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
}

type publishType struct {
	topic   string
	payload string
}

var (
	client            mqtt.Client
	logDirectory      string
	mqttBroker        string
	fullLogFileName   string
	updateChan        chan updateType
	deviceBackChan    chan string
	publishChan       chan publishType
	seasonStart       time.Time
	seasonEnd         time.Time
	globalEnable      bool
	verboseLog        bool
	regionMap         map[string]map[string]string // map region to a set of devices
	deviceMap         map[string]deviceType        // map a device name to its region
	lightLevel        int
	debug             bool
	lastPublish       time.Time
	stateMachineDefer time.Duration
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

	seasonStart, _ = time.Parse("2006-Jan-02 MST", "2018-Nov-01 EDT")
	seasonEnd, _ = time.Parse("2006-Jan-02 MST", "2100-Jan-06 EDT")
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

// All mqtt messages about christmas are handled here
var christmasHandler mqtt.MessageHandler = func(client mqtt.Client, msg mqtt.Message) {
	payload := string(msg.Payload())
	topic := string(msg.Topic())
	topicComponents := strings.Split(topic, "/")

	if debug {
		fmt.Printf("christmas message: %s %s\n", topic, payload)
	}

	switch topicComponents[1] {
	case "season":
		// get the mm/dd stuff and turn it into time.Time
		mmdd := strings.Split(payload, "/")
		if len(mmdd) == 2 {
			now := time.Now()
			year := now.Year()
			month, err := strconv.ParseInt(mmdd[0], 10, 32)
			if err != nil {
				return
			}
			if month < 7 {
				year++
			}
			day, err := strconv.ParseInt(mmdd[1], 10, 32)
			if err != nil {
				return
			}
			loc, _ := time.LoadLocation("Local")
			seasonTime := time.Date(year, time.Month(month), int(day), 0, 0, 0, 0, loc)
			switch topicComponents[2] {
			case "start":
				seasonStart = seasonTime
				if verboseLog {
					logMessage(fmt.Sprintf("Season Start set to %v", seasonStart))
				}
			case "end":
				seasonEnd = seasonTime
				if verboseLog {
					logMessage(fmt.Sprintf("Season End set to %v", seasonEnd))
				}
			}
		}
	case "enable":
		switch payload {
		case "true":
			globalEnable = true
			logMessage("Christmas control enabled")
		case "false":
			globalEnable = false
			logMessage("Christmas control disabled")
		}
	default:
		var update updateType
		update.update = "region"
		update.region = topicComponents[1]
		update.value1 = topicComponents[2]
		update.value2 = payload
		updateChan <- update

		if update.value1 == "devices" {
			for _, device := range strings.Split(payload, ",") {
				update.update = "device"
				update.value1 = device
				updateChan <- update
			}
		}
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
 * All action requests come here and are serialized that way
 */
func updater(con context.Context) {
	if debug {
		fmt.Println("Updater running")
	}

	// every so often wake up whether we've recieved any thing to do or not.
	timeoutDuration, _ := time.ParseDuration("1m")
	timeoutContext, _ := context.WithTimeout(con, timeoutDuration)
	for {
		select {
		case update := <-updateChan:
			if debug {
				fmt.Printf("Update recieved: %s\n", update.update)
			}
			switch update.update {
			case "region":
				region, ok := regionMap[update.region]
				if !ok {
					region = make(map[string]string)
				}
				region[update.value1] = update.value2
				regionMap[update.region] = region

			case "light":
				l, err := strconv.ParseInt(update.value1, 10, 32)
				if err == nil {
					lightLevel = int(l)
				}
			case "device":
				device, ok := deviceMap[update.value1]
				device.region = update.region
				if !ok {
					device.button = "false"
					device.outlet = "false"
				}
				deviceMap[update.value1] = device
				deviceBackChan <- update.value1 // tell main thread to subscribe

			case "outlet":
				device, ok := deviceMap[update.value1]
				if ok {
					device.outlet = update.value2
					deviceMap[update.value1] = device
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
			timeoutContext, _ = context.WithTimeout(con, timeoutDuration)
			stateMachine(client)
		case <-timeoutContext.Done():
			if debug {
				fmt.Println("Updater timeout")
			}
			timeoutContext, _ = context.WithTimeout(con, timeoutDuration)
			stateMachine(client)
		}
	}
}

// parse "hh:mm" spec
func parsehhmm(hhmm string, defaultHour int) (int, int) {
	if debug {
		fmt.Println("\t\t\tParsing: " + hhmm)
	}
	hc := strings.Split(hhmm, ":")
	if len(hc) != 2 {
		return defaultHour, 0
	}

	hour, err := strconv.ParseInt(hc[0], 10, 32)
	if err != nil {
		return defaultHour, 0
	}

	min, err := strconv.ParseInt(hc[1], 10, 32)
	if err != nil {
		return defaultHour, 0
	}

	if hour < 0 || hour > 23 || min < 0 || min > 59 {
		return defaultHour, 0
	}

	return int(hour), int(min)
}

// takes a specification in the form of "hh:mm" and decides when that is
func hhmmWindow(now time.Time, spec string, defaultHour int) time.Time {
	hour, minute := parsehhmm(spec, defaultHour)

	// times before noon are assumed to be tomorrow
	if hour < 12 {
		hour += 24
	}

	loc, _ := time.LoadLocation("Local")
	dayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)

	return dayStart.Add(time.Duration(hour) * time.Hour).Add(time.Duration(minute) * time.Minute)
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
	if debug {
		fmt.Println("State Machine Running")
	}
	// First, acknowledge button pushes
	for deviceName, device := range deviceMap {
		if device.button == "true" {
			if verboseLog {
				logMessage(fmt.Sprintf("button on device %s pushed", deviceName))
			}
			var p publishType
			p.topic = fmt.Sprintf("devices/%s/button/button/set", deviceName)
			p.payload = "false"
			publishChan <- p
		}
	}

	// Is Chistmas enabled?
	if !globalEnable {
		return
	}

	// Are we in the season?
	now := time.Now()
	if now.Before(seasonStart) || now.After(seasonEnd) {
		return
	}

	// If we've just published some stuff then don't run the state machine
	if time.Since(lastPublish) < stateMachineDefer {
		return
	}

	if debug {
		fmt.Println("\tEnabled and in season")
		fmt.Println("\tLight level is", lightLevel)
	}

	// For each region
	for regionName, region := range regionMap {
		// Are we in the window when the lights should be on?
		var start, end bool
		start = false
		end = false

		if debug {
			fmt.Println("\tRegion: ", regionName)
		}

		startString := region["window-start"]
		if startString == "light" {
			start = lightLevel < 4 && now.Hour() >= 12
		} else {
			start = now.After(hhmmWindow(now, startString, 17))
		}
		end = now.Before(hhmmWindow(now, region["window-end"], 11+12))

		inWindow := start && end
		if debug {
			fmt.Println("\t\tIn window:", inWindow)
		}

		// handle button pushes and automatic vs manual states
		for deviceName, device := range deviceMap {
			if device.region == regionName && device.button == "true" {
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
				regionMap[regionName] = region

				if debug {
					fmt.Printf("\t\t%s[\"control\"] set to %s\n", regionName, region["control"])
				}
			}
		}

		// If manual control has expired, return to automatic control
		if inWindow && region["control"] == "manual-o" {
			region["control"] = "auto"
			regionMap[regionName] = region
		}

		if !inWindow && region["control"] == "manual-i" {
			region["control"] = "auto"
			regionMap[regionName] = region
		}

		// Calculate whether the lights in this region should be on.
		shouldBeOn := inWindow
		if region["control"] == "manual-i" || region["control"] == "manual-o" {
			shouldBeOn = !shouldBeOn
		}

		if debug {
			fmt.Println("\t\tlights should be on:", shouldBeOn)
		}

		// Now, see if this matches the public state
		state, ok := region["state"]
		if !ok || shouldBeOn && state != "on" || !shouldBeOn && state == "on" {
			state = "off"
			if shouldBeOn {
				state = "on"
			}
			// don't need to update the region map, as we'll recieve the message we are about to publish

			var p publishType
			p.topic = fmt.Sprintf("christmas/%s/state", regionName)
			p.payload = state
			publishChan <- p
			logMessage(fmt.Sprintf("state in region %s set to %s", regionName, state))
		}

		// for each device, check whether its state matches the desired state
		// and set the device if necessary
		for deviceName, device := range deviceMap {
			if device.region == regionName {
				var p publishType
				p.topic = fmt.Sprintf("devices/%s/outlet/on/set", deviceName)
				if device.outlet == "true" && !shouldBeOn {
					p.payload = "false"
					publishChan <- p
					if verboseLog {
						logMessage(fmt.Sprintf("device %s in region %s set to off", deviceName, regionName))
					}
				}
				if device.outlet == "false" && shouldBeOn {
					p.payload = "true"
					publishChan <- p
					if verboseLog {
						logMessage(fmt.Sprintf("device %s in region %s set to on", deviceName, regionName))
					}
				}
			}
		}
	}
}

func main() {

	go updater(context.Background())

	//mqtt.DEBUG = log.New(os.Stdout, "", 0)
	logMessage("Christmas Daemon started")
	logMessage("mqtt broker = " + mqttBroker)
	mqtt.ERROR = log.New(os.Stdout, "", 0)
	opts := mqtt.NewClientOptions().AddBroker(mqttBroker).SetClientID("christmas-daemon")
	opts.SetKeepAlive(60 * time.Second)
	opts.SetPingTimeout(1 * time.Second)

	client = mqtt.NewClient(opts)
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		panic(token.Error())
	}

	if token := client.Subscribe("christmas/#", 0, christmasHandler); token.Wait() && token.Error() != nil {
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
				fmt.Println("Publishing ", pubRequest.topic, " : ", pubRequest.payload)
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
