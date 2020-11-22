/*
 * Log the environment.
 * Intent is to create a long term data capture of the environment for later analysis.
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
const defaultLogFileName = "HomeEnvironment"
const myName = "Environment Logger"
const defaultLogInterval = 15 // minutes
const defaultMqttBroker = "tcp://DanielPi3:1883"

type stateUpdateType struct {
	name  string
	state string
}

type valueUpdateType struct {
	name  string
	value float64
}

var (
	logDirectory    string
	fullLogFileName string
	mqttBroker      string
	logInterval     time.Duration
	updateChannel   chan interface{}
)

var stateMap = map[string]bool{
	"alarm-state": true,
}

var sensorMap = map[string]bool{
	"outdoor-temp": true,
	"outdoor-lux":  true,
}

type sensorDataType struct {
	sumValues float64
	numValues int
}

var sensorData map[string]sensorDataType

// Process the environment
// Initialize a few data structures
func init() {
	var i int64

	logDirectory = os.Getenv("LOGDIR")
	if len(logDirectory) < 1 {
		logDirectory = defaultLogDirectory
	}

	logFileName := os.Getenv("LOGFILENAME")
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

	mqttBroker = os.Getenv("MQTTBROKER")
	if len(mqttBroker) < 1 {
		mqttBroker = defaultMqttBroker
	}

	updateChannel = make(chan interface{})
	sensorData = make(map[string]sensorDataType)
}

/*
 * This handles messages from the mqtt client.
 * I believe that multiple instances of this can be called on different threads
 */
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
		var update stateUpdateType
		update.name = sensor
		update.state = payload
		updateChannel <- update
		return
	}

	// Is this a known sensor?
	if _, ok := sensorMap[sensor]; ok {
		var update valueUpdateType
		update.name = sensor
		v, err := strconv.ParseFloat(payload, 64)
		if err != nil {
			return
		}
		update.value = v
		updateChannel <- update
		return
	}

	// If we don't know what this is, do nothing.
}

/*
 * This routine returns a context that will timeout when we are next
 * supposed to dump sensor data to the log.  The timeouts are naturally
 * aligned.  For example, if we are doing timeouts once an hour, then they will
 * occur on the hour.  Alignments occur relative to last midnight.
 *
 * The global variable logInterval controls how often we do this.
 */
func heartbeat(con context.Context) context.Context {
	// First, compute when last midnight was
	year, month, date := time.Now().Date()
	loc, _ := time.LoadLocation("Local")
	lastMidnight := time.Date(year, month, date, 0, 0, 0, 0, loc)
	// Now, how many intervals since then
	ni := time.Since(lastMidnight) / logInterval
	// One more interval until the deadline
	c, _ := context.WithDeadline(con, lastMidnight.Add((ni+1)*logInterval))
	return c
}

/*
 * This routine runs as its own thread, recieving updates from the
 * message handler and generating log messages as appropriate.
 * It also uses contexts to generate a periodic heartbeat for dumping
 * sensor averages to the log.
 */
func updater(con context.Context) {
	timeoutContext := heartbeat(con)
	dataLogger("logdaemon", "startup")

	// Loop forever getting updates and heartbeats
	for {
		select {
		case data := <-updateChannel:
			switch update := data.(type) {
			case stateUpdateType:
				// For state variables, just log their changed state
				dataLogger(update.name, update.state)
			case valueUpdateType:
				// For sensor variables, we average them until the next heartbeat
				var s sensorDataType
				s, ok := sensorData[update.name]
				if !ok {
					s.sumValues = 0
					s.numValues = 0
				}
				s.sumValues += update.value
				s.numValues++
				sensorData[update.name] = s
			}
		case <-timeoutContext.Done():
			// reset the heartbeat context
			timeoutContext = heartbeat(con)

			// Loop over all known sensors, outputing stuff and zeroing out
			for name, data := range sensorData {
				if data.numValues <= 0 {
					dataLogger(name, "no-data")
				} else {
					dataLogger(name, strconv.FormatFloat(data.sumValues/float64(data.numValues), 'f', 1, 64))
				}
				data.numValues = 0
				data.sumValues = 0.
				sensorData[name] = data
			}
		}
	}
}

func main() {

	//mqtt.DEBUG = log.New(os.Stdout, "", 0)
	logMessage("mqtt broker = " + mqttBroker)
	mqtt.ERROR = log.New(os.Stdout, "", 0)
	opts := mqtt.NewClientOptions().AddBroker(mqttBroker).SetClientID("envlogger-daemon")
	opts.SetKeepAlive(60 * time.Second)
	opts.SetPingTimeout(1 * time.Second)

	client := mqtt.NewClient(opts)
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		logMessage(fmt.Sprintf("Client Connect Failed: %f", token.Error()))
		panic(token.Error())
	}
	logMessage("environment data logger started")

	// start up the data aggregator
	go updater(context.Background())

	if token := client.Subscribe("environment/#", 0, handler); token.Wait() && token.Error() != nil {
		logMessage(fmt.Sprintf("Client Subscribe Failed: %f", token.Error()))
		panic(token.Error())
	}

	// sleep forever
	for {
		time.Sleep(1 * time.Second)
	}
}

func dataLogger(event, value string) {
	logMessage(", " + event + ", " + value)
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
