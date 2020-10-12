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
		Publish to: devices/led-0001/led/on
 *
*/

package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
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
 */
var alarmStates = map[string]bool{
	"disarmed":        true,
	"armed-stay":      true,
	"alarmed-burglar": true,
	"alarmed-fire":    true,
	"armed-away":      true,
}

var r1handler mqtt.MessageHandler = func(client mqtt.Client, msg mqtt.Message) {
	payload := string(msg.Payload())

	if _, ok := alarmStates[payload]; ok {
		client.Publish("environment/alarm-state", 0, true, payload)
		logMessage("Alarm state: " + payload)
	} else {
		logMessage("Invalid alarm state: " + payload)
	}
}

/*
var r23handler mqtt.MessageHandler = func(client mqtt.Client, msg mqtt.Message) {
	topic := msg.Topic()
	payload := string(msg.Payload())
}
*/

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

	if token := client.Subscribe("devices/alarm-state-0001/alarm-state/state", 0, r1handler); token.Wait() && token.Error() != nil {
		fmt.Println(token.Error())
		os.Exit(1)
	}

	/*
		if token := c.Subscribe("environment/alarm-state", 0, r23handler); token.Wait() && token.Error() != nil {
			fmt.Println(token.Error())
			os.Exit(1)
		}
	*/

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

	formattedMsg := time.Now().Format("Mon Jan 2 15:04:05 2006") + "  " + m

	_, err = f.WriteString(formattedMsg)
	if err != nil {
		log.Printf("Logger: Error writing to file %s.  err = %v\n", fullLogFileName, err)
		return
	}
}
