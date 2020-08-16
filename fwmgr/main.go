/*
 * This program manages firmware on HOMIE devices
 */

package main

import (
	"fmt"
	"log"
	"os"
	"time"
	"strings"
	//"strconv"
	"regexp"
	"context"
	"flag"

	"github.com/eclipse/paho.mqtt.golang"
)

var (
	flagu bool
	flagl bool
	flagD bool
	flagF bool
	flagd string
	flagf string
	flagdPresent bool
	flagfPresent bool
)

var (
	epoch time.Time
	deviceMap map[string]map[string]string		// all the properties of all the devices
	deviceMatch * regexp.Regexp = regexp.MustCompile("devices/([a-zA-Z0-9\\-]+)")
	deviceSubTopicMatch * regexp.Regexp = regexp.MustCompile("devices/[a-zA-Z0-9\\-]+/(.*)")
	timeoutContext context.Context
	timeoutChannel chan int = make(chan int)
)

func init() {
	epoch, _ = time.Parse("2006-Jan-02 MST", "2018-Nov-01 EDT")
	deviceMap = make(map[string]map[string]string)

	flag.BoolVar(&flagu, "u", false, "upload firmware")
	flag.BoolVar(&flagl, "l", false, "list devices or device (with -d) or verify a firmware file (with -f)")
	flag.BoolVar(&flagD, "D", false, "debugging")
	flag.BoolVar(&flagF, "F", false, "clear our OTA crap and reset the device")
	flag.StringVar(&flagd, "d", "", "name of a device")
	flag.StringVar(&flagf, "f", "", "firmware file name")

	flag.Parse()
	flagdPresent = (flagd != "")
	flagfPresent = (flagf != "")
}

// Call the cancel function after a deadline.  Each time any value is received
// on the channel, the deadline is extended.
func timeoutRoutine(c context.Context, cf context.CancelFunc, d time.Duration, ch chan int) {
	for {
		subContext, cfl := context.WithTimeout(c, d)
		select {
		case <-subContext.Done():
			cfl()
			cf()
			return
		case <-ch:
			cfl()
		}
	}
}

var f1 mqtt.MessageHandler = func(client mqtt.Client, msg mqtt.Message) {
	topic := msg.Topic()
	payload := string(msg.Payload())

	// Ignore broadcast messages
	if strings.Contains(topic, "$broadcast") {
		return
	}

	if strings.Contains(topic, "firmware") {
		payload = "(suppressed)"
	}

	// tell the world we are still working
	timeoutChannel <- 0

	device := deviceMatch.FindStringSubmatch(topic)[1]
	deviceSubTopic := deviceSubTopicMatch.FindStringSubmatch(topic)[1]
	if  deviceMap[device] == nil {
		deviceMap[device] = make(map[string]string)
	}
	deviceMap[device][deviceSubTopic] = payload
}

// get all the persistent messages and build a map of everything we know about everybody
func getDevices() {
	opts := mqtt.NewClientOptions().AddBroker("tcp://127.0.0.1:1883").SetClientID("fw-test")
	opts.SetKeepAlive(60 * time.Second)
	opts.SetDefaultPublishHandler(f1)
	opts.SetPingTimeout(1 * time.Second)

	c := mqtt.NewClient(opts)
	if token := c.Connect(); token.Wait() && token.Error() != nil {
		panic(token.Error())
	}

	if token := c.Subscribe("devices/#", 0, nil); token.Wait() && token.Error() != nil {
		fmt.Println(token.Error())
		os.Exit(1)
	}

	// Set up to wait for 1 second after last message is received
	timeoutContext, cf := context.WithCancel(context.Background())
	go timeoutRoutine(timeoutContext, cf, time.Second, timeoutChannel)
	select {
	case <-timeoutContext.Done():
	}

	if token := c.Unsubscribe("devices/#"); token.Wait() && token.Error() != nil {
		fmt.Println(token.Error())
		os.Exit(1)
	}

	c.Disconnect(250)

	time.Sleep(1 * time.Second)
}

// Print out the current firmware info for a device
func deviceInfo(device string) {
	info := []string{ "name", "version", "checksum" }

	dmap, ok := deviceMap[device]
	if !ok {
		fmt.Printf("Device %s not found in mqtt database\n", device)
		return
	}

	fmt.Printf("%s:\n", device)
	for _, field := range info {
		v, ok := dmap["$fw/" + field]
		if !ok {
			fmt.Printf("\tFW %s is missing\n", field)
		} else {
			fmt.Printf("\tFW %s: %s\n", field, v)
		}
	}
}

func fileInfo(file string) {
	fmt.Printf("File info for file %s is not yet implemented\n", file)
}

func main() {
	//mqtt.DEBUG = log.New(os.Stdout, "", 0)
	mqtt.ERROR = log.New(os.Stdout, "", 0)

	getDevices()

	// multiple modes
	if flagl {
		if flagfPresent {
			fileInfo(flagf)
		} else if flagdPresent {
			deviceInfo(flagd)
		} else {
			fmt.Printf("Found these devices:\n")
			for k, _ := range deviceMap {
				deviceInfo(k)
			}
		}
	} else {
		fmt.Printf("Only list mode (\"-l\") is presently implemented\n")
	}
}
