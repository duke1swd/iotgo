/*
 * First generation automation daemon.
 *
 * Ideally this is all driven by rules and such.
 * For now these items will be hard-coded:
 * 	R1: The alarm state is propagated to an environmental state
 		Subscribe to: devices/alarm-state-0001/alarm-state/state
		Publish to:   environment/alarm-state
 *	R2: The alarm state triggers changes in the state of ZoneMinder
		Subscribe to: environment/alarm-state
		Publish to: Zoneminder APIs
 *	R3: The alarm state turns on or off an LED indicator
		Subscribe to: environment/alarm-state
		Publish to: devices/led-0001/led/on/set
 *
*/

package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/eclipse/paho.mqtt.golang"
)

const defaultLogDirectory = "/var/log"
const logFileName = "HomeAutomationLog"

var (
	client          mqtt.Client
	logDirectory    string
	fullLogFileName string
)

func init() {
	logDirectory = os.Getenv("LOGDIR")
	if len(logDirectory) < 1 {
		logDirectory = defaultLogDirectory
	}
	fullLogFileName = filepath.Join(logDirectory, logFileName)
}

/*
 * Rule 1: subscribe to the device level alarm state
 * and publish to the environment level.
 *
 * The value of each alarm state is a code sent to the LED controller
 */
var alarmStates = map[string]int{
	"disarmed":        0,
	"armed-stay":      10,
	"alarmed-burglar": 5,
	"alarmed-fire":    5,
	"armed-away":      1,
	"unknown":         0,
}

const (
	DET_UNKNOWN int = 1 + iota
	DET_ONLINE
	DET_OFFLINE
)

var detState int = DET_UNKNOWN
var detStateDetailed string = "unknown"
var lastAlarmState string = "unknown"

var stateMap = map[string]int{
	"init":         DET_OFFLINE,
	"ready":        DET_ONLINE,
	"disconnected": DET_OFFLINE,
	"sleeping":     DET_OFFLINE,
	"lost":         DET_OFFLINE,
	"alert":        DET_OFFLINE,
}

// Since both r1a and r1b can happen simultaneously there are race conditions her.
// Right thing to do would be to push the detector state changes and the alarm state
// changes to a go routine to serialize them. Maybe next version.

// Changes in the alarm state come here.
var r1ahandler mqtt.MessageHandler = func(client mqtt.Client, msg mqtt.Message) {
	payload := string(msg.Payload())

	if _, ok := alarmStates[payload]; ok {
		if detState == DET_ONLINE {
			client.Publish("environment/alarm-state", 0, true, payload)
		}
		logMessage("Alarm state: " + payload)
		lastAlarmState = payload
	} else {
		logMessage("Invalid device alarm state: " + payload)
	}
}

// Changes in whether the alarm detector is on line come here
var r1bhandler mqtt.MessageHandler = func(client mqtt.Client, msg mqtt.Message) {
	var newState int

	payload := string(msg.Payload())

	if detStateDetailed != payload {
		detStateDetailed = payload
		logMessage("Alarm Detector online state: " + payload)
	}

	newState, ok := stateMap[payload]
	if !ok {
		newState = DET_UNKNOWN
	}

	if newState != detState {
		detState = newState
		if newState == DET_ONLINE {
			client.Publish("environment/alarm-state", 0, true, lastAlarmState)
		} else {
			client.Publish("environment/alarm-state", 0, true, "unknown")
		}
	}

}

var r23handler mqtt.MessageHandler = func(client mqtt.Client, msg mqtt.Message) {

	payload := string(msg.Payload())
	ledValue, ok := alarmStates[payload]
	log.Printf("r23 payload = %s\n", payload)
	if !ok {
		logMessage("Invalid environment alarm state: " + payload)
		return
	}

	// Rule 2: Interior cameras are on if unless the alarm is off
	if payload == "disarmed" {
		zoneHome()
	} else {
		zoneAway()
	}

	// Rule 3: Display alarm state on the LED.
	client.Publish("devices/led-0001/led/on/set", 0, true, strconv.Itoa(ledValue))
}

// Turn on the interior cameras
func zoneAway() {
	zoneState("Away")
}

// Turn off the interior cameras
func zoneHome() {
	zoneState("Home")
}

// Set a run state in Zoneminder
func zoneState(state string) {
}

func main() {

	//mqtt.DEBUG = log.New(os.Stdout, "", 0)
	mqtt.ERROR = log.New(os.Stdout, "", 0)
	opts := mqtt.NewClientOptions().AddBroker("tcp://127.0.0.1:1883").SetClientID("automation-daemon")
	opts.SetKeepAlive(60 * time.Second)
	opts.SetPingTimeout(1 * time.Second)

	client = mqtt.NewClient(opts)
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		panic(token.Error())
	}

	logMessage("home automation daemon started")

	if token := client.Subscribe("devices/alarm-state-0001/alarm-state/state", 0, r1ahandler); token.Wait() && token.Error() != nil {
		fmt.Println(token.Error())
		os.Exit(1)
	}

	if token := client.Subscribe("devices/alarm-state-0001/$state", 0, r1bhandler); token.Wait() && token.Error() != nil {
		fmt.Println(token.Error())
		os.Exit(1)
	}

	if token := client.Subscribe("environment/alarm-state", 0, r23handler); token.Wait() && token.Error() != nil {
		fmt.Println(token.Error())
		os.Exit(1)
	}

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
