/*
 * Log the environment.
 * Intent is to create a long term data capture of the environment for later analysis.
*/

package main

import (
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
const defaultLogFileName = "HomeEnvironment"
const myName = "Environment Logger"
const defaultLogInterval = 15	// minutes

var (
	logDirectory    string
	fullLogFileName string
	logInterval time.Duration
)

var stateMap = map[string]bool {
	"alarm-state": true,
}

// Process the environment
func init() {
	var i int64

	logDirectory = os.Getenv("LOGDIR")
	if len(logDirectory) < 1 {
		logDirectory = defaultLogDirectory
	}

	logFileName := os.Getenv("LOGNAME")
	if len(logFileName) < 1 {
		logFileName = defaultLogFileName
	}
	fullLogFileName = filepath.Join(logDirectory, logFileName)

	t := os.Getenv("INTERVAL")
	if len(t) < 1 {
		i = defaultLogInterval
	} else {
		j, err := strconv.ParseInt(t, 10, 32)
		if err != nil {
			j = defaultLogInterval
		}
		i = j
	}
	logInterval = time.Duration(i) * time.Minute
}

var handler mqtt.MessageHandler = func(client mqtt.Client, msg mqtt.Message) {
	topic := string(msg.Topic())
	payload := string(msg.Payload())

	// for now, ignore the time pulse
	if strings.Contains(topic, "IOTtime") {
		return
	}

	// Split out the topic.  We can only process top level under "environment"
	topicComponents := strings.Split(topic, "/")
	if len(topicComponents) != 2 {
		return
	}
	sensor := topicComponents[1]

	// Is this a known state variable?
	if _, ok := stateMap[sensor]; ok {
		dataLog(sensor, payload)
		return
	}
}

func main() {

	//mqtt.DEBUG = log.New(os.Stdout, "", 0)
	mqtt.ERROR = log.New(os.Stdout, "", 0)
	opts := mqtt.NewClientOptions().AddBroker("tcp://127.0.0.1:1883").SetClientID("automation-daemon")
	opts.SetKeepAlive(60 * time.Second)
	opts.SetPingTimeout(1 * time.Second)

	client := mqtt.NewClient(opts)
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		panic(token.Error())
	}
	logMessage("environment data logger started")

	if token := client.Subscribe("environment/#", 0, handler); token.Wait() && token.Error() != nil {
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
		log.Printf("%s: Cannot open for writing log file %s. err = %v", myName, fullLogFileName, err)
		return
	}
	defer f.Close()

	formattedMsg := time.Now().Format("Mon Jan 2 15:04:05 2006") + "  " + m + "\n"

	_, err = f.WriteString(formattedMsg)
	if err != nil {
		log.Printf("%s: Error writing to file %s.  err = %v\n", myName, fullLogFileName, err)
		return
	}
}
