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

var (
	client          mqtt.Client
	logDirectory    string
	mqttBroker      string
	fullLogFileName string
	epoch           time.Time
	updateChan      chan updateType
	seasonStart     time.Time
	seasonEnd       time.Time
	globalEnable    bool
	verboseLog      bool
	regionMap       map[string]map[string]string
	deviceMap       map[string]deviceType // map a device name to its region
	modeConfig      bool
	lightLevel      int
	debug           bool
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

	epoch, _ = time.Parse("2006-Jan-02 MST", "2018-Nov-01 EDT")
	seasonStart, _ = time.Parse("2006-Jan-02 MST", "2018-Nov-01 EDT")
	seasonEnd, _ = time.Parse("2006-Jan-02 MST", "2100-Jan-06 EDT")

	updateChan = make(chan updateType)
	regionMap = make(map[string]map[string]string)
	deviceMap = make(map[string]deviceType)
	lightLevel = 0
	globalEnable = false
}

// All mqtt messages about christmas are handled here
var christmasHandler mqtt.MessageHandler = func(client mqtt.Client, msg mqtt.Message) {
	payload := string(msg.Payload())
	topic := string(msg.Topic())
	topicComponents := strings.Split(topic, "/")

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
			if verboseLog {
				logMessage("Christmas control enabled")
			}
		case "false":
			globalEnable = false
			if verboseLog {
				logMessage("Christmas control disabled")
			}
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

				sub := "devices/" + device + "/#"
				if token := client.Subscribe(sub, 0, deviceHandler); token.Wait() && token.Error() != nil {
					logMessage(fmt.Sprintf("Failed to subscribe to %s.  Err=%v", sub, token.Error()))
				} else if verboseLog {
					logMessage(fmt.Sprintf("Subscribed to %s", sub))
				}
			}
		}
	}
}

// All mqtt messages about light level are handled here
var lightHandler mqtt.MessageHandler = func(client mqtt.Client, msg mqtt.Message) {
	payload := string(msg.Payload())

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

	topicComponents := strings.Split(topic, "/")
	if len(topicComponents) < 4 {
		return
	}

	device := topicComponents[1]
	update.value1 = device
	update.value2 = payload

	if topicComponents[2] == "outlet" && topicComponents[3] == "on" {
		update.update = "outlet"
		updateChan <- update
	}

	if topicComponents[2] == "button" && topicComponents[3] == "button" {
		var update updateType
		update.update = "button"
		updateChan <- update
	}
}

/*
 * All action requests come here and are serialized that way
 */
func updater(con context.Context, client mqtt.Client) {

	// every so often wake up whether we've recieved any thing to do or not.
	timeoutDuration, _ := time.ParseDuration("1m")
	timeoutContext, _ := context.WithTimeout(con, timeoutDuration)
	for {
		select {
		case update := <-updateChan:
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
				}
			}
			timeoutContext, _ = context.WithTimeout(con, timeoutDuration)
			stateMachine(client)
		case <-timeoutContext.Done():
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
			topic := fmt.Sprintf("devices/%s/button/button/set", deviceName)
			client.Publish(topic, 0, true, "false")
			if verboseLog {
				logMessage(fmt.Sprintf("button on device %s pushed", deviceName))
			}
		}
	}

	// don't run the state machine until the state is fully loaded
	if modeConfig {
		return
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
			start = lightLevel < 2 && now.Hour() >= 12
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

			topic := fmt.Sprintf("christmas/%s/state", regionName)
			client.Publish(topic, 0, true, state)
			if verboseLog {
				logMessage(fmt.Sprintf("state in region %s set to %s", regionName, state))
			}
		}

		// for each device, check whether its state matches the desired state
		// and set the device if necessary
		for deviceName, device := range deviceMap {
			if device.region == regionName {
				topic := fmt.Sprintf("devices/%s/outlet/on/set", deviceName)
				if device.outlet == "true" && !shouldBeOn {
					client.Publish(topic, 0, true, "false")
					if verboseLog {
						logMessage(fmt.Sprintf("device %s in region %s set to off", deviceName, regionName))
					}
				}
				if device.outlet == "false" && shouldBeOn {
					client.Publish(topic, 0, true, "true")
					if verboseLog {
						logMessage(fmt.Sprintf("device %s in region %s set to on", deviceName, regionName))
					}
				}
			}
		}
	}
}

func main() {

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

	logMessage("Christmas daemon started")

	if token := client.Subscribe("christmas/#", 0, christmasHandler); token.Wait() && token.Error() != nil {
		fmt.Println(token.Error())
		os.Exit(1)
	}

	if token := client.Subscribe("environment/outdoor-light", 0, lightHandler); token.Wait() && token.Error() != nil {
		fmt.Println(token.Error())
		os.Exit(1)
	}

	modeConfig = true
	go updater(context.Background(), client)
	time.Sleep(1 * time.Second)
	modeConfig = false

	// sleep forever
	for {
		time.Sleep(1 * time.Second)
	}
}

func logMessage(m string) {
	f, err := os.OpenFile(fullLogFileName, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		log.Printf("Logger: Cannot open for writing log file %s. err = %v", fullLogFileName, err)
		return
	}
	defer f.Close()

	formattedMsg := time.Now().Format("Mon Jan 2 15:04:05 2006") + "  " + m + "\n"

	// if debugging just send to stdout
	if debug {
		fmt.Print(formattedMsg)
	} else {
		_, err = f.WriteString(formattedMsg)
		if err != nil {
			log.Printf("Logger: Error writing to file %s.  err = %v\n", fullLogFileName, err)
			return
		}
	}
}
