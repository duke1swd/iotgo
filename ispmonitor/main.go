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
	"github.com/duke1swd/iotgo/logQueue"
	"log"
	"time"
)

const topicID = "Logs" // Log messages go to topic Logs.
const service = "ISPMonitor"
const defaultLocation = "unknown"
const defaultRouter = "192.168.1.1"
const defaultProjectID = "iot-services-274518" // This is the IOT Services project
const defaultPollInterval = 300                // 5 minutes

var (
	oldNow int64
	seqn   int
	epoch  time.Time
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
	logStateInternetUp
	logStateInternetDown
	logStateWiFiDown
)

func (m logMessage) String() string {
	return [...]string{
		"Hello World!",
		"Internet Down for %d seconds",
		"WiFi Down for %d seconds",
		"Internet Up for %d seconds",
		"Router Reset try %d",
		"Modem Reset try %d",
		"Internet Up",
		"Internet Down",
		"WiFi Down",
	}[m]
}

func main() {
	var err error

	currentState = stateBooting
	stateEntryTime = time.Now()

	ctx, cxf := context.WithCancel(context.Background())
	defer cxf()

	myPublishInit(ctx)

	err = logQueue.Start(ctx, publishDeferredMessage)
	if err != nil {
		log.Fatalf("failed to start log queue. Err = %v", err)
	}

	mainLoop(ctx)
}
