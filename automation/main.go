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

 *	R4: The lux value is propogated to an environmental state
		Subscribe to:
			devices/environ-0001/lux/lux
			devices/environ-0001/lux/time-last-update
		Publish to: environment/outdoor-lux
		Publish every data point received, with hysteresis of 2 lux.
		Supress publication whenever the "time-last-update" is more than 3 minutes old.
		If the data is more than 10 minutes old, publish "unknown" as the lux value

 *	R5: The temp value is propogated to an environmental state
		Subscribe to:
			devices/environ-0001/temp/temp
			devices/environ-0001/temp/time-last-update
		Publish to: environment/outdoor-temp
		Publish every data point received, with hysteresis of .5 degrees
		Supress publication whenever the "time-last-update" is more than 3 minutes old.
		If the data is more than 10 minutes old, publish "unknown" as the temp value

 *
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
const logFileName = "HomeAutomationLog"

var (
	client          mqtt.Client
	logDirectory    string
	fullLogFileName string
	sensorTime      map[string]time.Time
	epoch           time.Time
	updateChan      chan interface{}
	sensorExpire    time.Duration
)

type sensorStateType struct {
	hysteresis float64
	lastValue  float64
	updateTime time.Time
	valueKnown bool
}

type sensorUpdateValueType struct {
	name  string
	value float64
}

type sensorUpdateTimeType struct {
	name       string
	updateTime time.Time
}

// This is the list of sensors.
var sensorStates = map[string]sensorStateType{
	"lux":  {2., 0., *new(time.Time), false},
	"temp": {.5, 0., *new(time.Time), false},
}

func init() {
	logDirectory = os.Getenv("LOGDIR")
	if len(logDirectory) < 1 {
		logDirectory = defaultLogDirectory
	}
	fullLogFileName = filepath.Join(logDirectory, logFileName)

	// set the time to a long time ago.
	for k, s := range sensorStates {
		s.updateTime, _ = time.Parse("2006-Jan-02 MST", "2000-Jan-01 EST")
		sensorStates[k] = s
	}

	epoch, _ = time.Parse("2006-Jan-02 MST", "2018-Nov-01 EDT")

	sensorExpire, _ = time.ParseDuration("120s") // sensor data invalide if not refreshed every minute or so
	updateChan = make(chan interface{})
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

var (
	detState         int    = DET_UNKNOWN
	detStateDetailed string = "unknown"
	lastAlarmState   string = "unknown"
)

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

// Changes in the outdoor environment come here

var r45subscriptions = []string{
	"devices/environ-0001/lux/lux",
	"devices/environ-0001/lux/time-last-update",
	"devices/environ-0001/temp/temp",
	"devices/environ-0001/temp/time-last-update",
}

var r45handler mqtt.MessageHandler = func(client mqtt.Client, msg mqtt.Message) {
	var (
		updateValue sensorUpdateValueType
		updateTime  sensorUpdateTimeType
	)
	topic := string(msg.Topic())
	payload := string(msg.Payload())

	topicComponents := strings.Split(topic, "/")
	sensor := topicComponents[2]

	switch topicComponents[3] {
	case sensor:
		// convert sensor value to a float64 and send it to the updater go routine
		t, err := strconv.ParseFloat(payload, 64)
		if err == nil {
			updateValue.name = sensor
			updateValue.value = t
			updateChan <- updateValue
		}
	case "time-last-update":
		// convert sensor last update time to a time.Time and send it to the updater go routine
		t, err := strconv.ParseInt(payload, 10, 64)
		if err == nil {
			updateTime.name = sensor
			updateTime.updateTime = epoch.Add(time.Duration(t) * time.Second)
			updateChan <- updateTime
		}
	}
}

/*
 * This function is a go routine that serializes updates to the sensorStates map.
 * It also implements a periodic timeout looking for sensors that have not updated in a while.
 */
func sensorUpdateHandler(con context.Context, client mqtt.Client) {

	// every so often wake up whether we've recieved any thing to do or not.
	timeoutDuration, _ := time.ParseDuration("1m")
	timeoutContext, _ := context.WithTimeout(con, timeoutDuration)
	for {
		select {
		case data := <-updateChan:
			switch update := data.(type) {
			case sensorUpdateValueType:
				s := sensorStates[update.name]
				s.lastValue = update.value
				s.valueKnown = true
				sensorStates[update.name] = s
				payload := strconv.FormatFloat(update.value, 'f', 1, 64)
				client.Publish("environment/outdoor-"+update.name, 0, true, payload)
			case sensorUpdateTimeType:
				s := sensorStates[update.name]
				s.updateTime = update.updateTime
				sensorStates[update.name] = s
			}
		case <-timeoutContext.Done():
			// once a minute we come here and look for sensors who's last update time
			// is too old and mark them as unknown.
			for name, s := range sensorStates {
				if time.Since(s.updateTime) > sensorExpire && s.valueKnown {
					s.valueKnown = false
					log.Printf("Sensor %s off line. Last update %v\n", name, s.updateTime)
					sensorStates[name] = s
					client.Publish("environment/outdoor-"+name, 0, true, "offline")
				}
			}
			timeoutContext, _ = context.WithTimeout(con, timeoutDuration)
		}
	}
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

	go sensorUpdateHandler(context.Background(), client)

	for _, s := range r45subscriptions {
		if token := client.Subscribe(s, 0, r45handler); token.Wait() && token.Error() != nil {
			fmt.Println(token.Error())
			os.Exit(1)
		}
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
