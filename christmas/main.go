/*
 * Christmas daemon.  See README
 */

package main

import (
	"context"
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
					region := make(map[string]string)
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
			stateMachine()
		case <-timeoutContext.Done():
			stateMachine()
			timeoutContext, _ = context.WithTimeout(con, timeoutDuration)
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

	_, err = f.WriteString(formattedMsg)
	if err != nil {
		log.Printf("Logger: Error writing to file %s.  err = %v\n", fullLogFileName, err)
		return
	}
}
