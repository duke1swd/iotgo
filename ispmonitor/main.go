/*
 * This program monitors the health of the Internet.
 * Every 5 minutes it publishes an I'm OK message to google's pub/sub.
 * If that fail, then things are no OK.  Various log mesages result.
 *
 * The key for talking to google must be set up in the environment variables.
 */

package main

import (
	"context"
	"log"
	"os"
	"time"

	"cloud.google.com/go/pubsub"
	"github.com/duke1swd/iotgo/logQueue"
)

const topicID = "Logs" // Log messages go to topic Logs.
const service = "ISPMonitor"
const defaultLocation = "unknown"
const defaultRouter = "192.168.1.1"

var (
	oldNow int64
	seqn   int
	epoch  time.Time
	topic  *pubsub.Topic
)

/*
 Declare the various log messages we use
*/
type logMessage int

const (
	logHelloWorld logMessage = iota
	logInternetDown
	logWiFiDown
	logLifeIsGood
	logWiFiReset
	logModemReset
)

func (m logMessage) String() string {
	return [...]string{
		"Hello World!",
		"Internet Down for %d seconds",
		"WiFi Down for %d seconds",
		"Internet Up for %d seconds",
		"Router Reset try %d",
		"Modem Reset try %d",
	}[m]
}

type state int

const (
	stateBooting = iota
	stateNoInternet
	stateNoWiFi
	stateGood
)

var (
	currentState   state
	stateEntryTime time.Time
	stateCounter   int
	attemptCounter int
	location       string
)

func main() {
	var err error

	currentState = stateBooting
	stateEntryTime = time.Now()

	location = os.Getenv("LOCATION")
	if len(location) < 1 {
		location = defaultLocation
	}

	ctx, cxf := context.WithCancel(context.Background())
	defer cxf()

	epoch, err = time.Parse("2006-Jan-02 MST", "2018-Nov-01 EDT")
	if err != nil {
		log.Fatalf("failed to get epoch. Err = %v", err)
	}

	// This is the IOT Services project
	projectID := "iot-services-274518"

	// Creates a client.
	client, err := pubsub.NewClient(ctx, projectID)
	if err != nil {
		log.Fatalf("ISP Monitor: Failed to create client: %v", err)
	}

	// Get a pointer to the topic object
	topic = client.Topic(topicID)
	if topic == nil {
		log.Fatalf("ISP Monitor: Failed to get topic: %s", topicID)
	}
	defer topic.Stop()

	// request to send messages immediately
	topic.PublishSettings.CountThreshold = 1

	err = logQueue.Start(ctx, publishDeferredMessage)
	if err != nil {
		log.Fatalf("failed to start log queue. Err = %v", err)
	}

	mainLoop(ctx)
}
