/*
 * This program can be used to clean messages from off-line devices out of mqtt
 */

package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/eclipse/paho.mqtt.golang"
)

var (
	flagl        bool
	flagL        bool
	flagD        bool
	flagc        string
	flagf        bool
	flagcPresent bool
	mqttClient   mqtt.Client
)

var (
	epoch               time.Time
	deviceMap           map[string]map[string]string // all the properties of all the devices
	deviceMatch         *regexp.Regexp               = regexp.MustCompile("devices/([a-zA-Z0-9\\-]+)")
	deviceSubTopicMatch *regexp.Regexp               = regexp.MustCompile("devices/[a-zA-Z0-9\\-]+/(.*)")
	timeoutContext      context.Context
	timeoutChannel      chan int = make(chan int)
	homieVersion        int      = 3
	otaChannel          chan int = make(chan int)
	returnedChecksum    string   = ""
)

func init() {
	epoch, _ = time.Parse("2006-Jan-02 MST", "2018-Nov-01 EDT")
	deviceMap = make(map[string]map[string]string)

	flag.BoolVar(&flagl, "l", false, "list devices that are state \"lost\"")
	flag.BoolVar(&flagL, "L", false, "list all devices and their state")
	flag.BoolVar(&flagD, "D", false, "debugging")
	flag.StringVar(&flagc, "c", "", "clear info for device")
	flag.BoolVar(&flagf, "f", false, "with -c, clears devices that are not state \"lost\"")

	flag.Parse()
	flagcPresent = (flagc != "")

	errors := 0

	if flagf && !flagcPresent {
		fmt.Printf("-f (Force) requires -c <dev>\n")
		errors += 1
	}

	if flagcPresent && (flagl || flagL) {
		fmt.Printf("cannot mix -c with -l or -L\n")
		errors += 1
	}

	if !flagcPresent && !flagl && !flagL {
		fmt.Println("must specify one of -c, -l, or -L")
		errors += 1
	}

	if errors > 0 {
		os.Exit(1)
	}
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
	if deviceMap[device] == nil {
		deviceMap[device] = make(map[string]string)
	}
	deviceMap[device][deviceSubTopic] = payload
}

func getClient() {
	opts := mqtt.NewClientOptions().AddBroker("tcp://127.0.0.1:1883").SetClientID("fw-test")
	opts.SetKeepAlive(60 * time.Second)
	opts.SetDefaultPublishHandler(f1)
	opts.SetPingTimeout(1 * time.Second)

	c := mqtt.NewClient(opts)
	if token := c.Connect(); token.Wait() && token.Error() != nil {
		panic(token.Error())
	}
	mqttClient = c
}

// get all the persistent messages and build a map of everything we know about everybody
func getDevices() {
	c := mqttClient

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
}

// List all devices
func deviceInfo() {
	for dev, properties := range deviceMap {
		state, ok := properties["$state"]
		if !ok {
			state = "<unknown>"
		}
		if flagL {
			fmt.Printf("%s: state=%s\n", dev, state)
		} else if state == "lost" {
			fmt.Println(dev)
		}
	}
}

// returns true on error
func clearDevice(device string) bool {
	properties, ok := deviceMap[device]

	if !ok {
		fmt.Printf("Device %s not found\n", device)
		return true
	}

	if !flagf && properties["$state"] != "lost" {
		fmt.Printf("Device %s not known to be offline\n", device)
		return true
	}
	fmt.Printf("Clearing device %s\n", device)

	for topic, _ := range properties {
		topic = "devices/" + device + "/" + topic
		if flagD {
			fmt.Printf("\t%s\n", topic)
		}
		publishToken := mqttClient.Publish(topic, 0, true, "")
		if publishToken.Wait() && publishToken.Error() != nil {
			panic(publishToken.Error())
		}
	}
	return false
}

func main() {
	var errors bool = false

	//mqtt.DEBUG = log.New(os.Stdout, "", 0)
	mqtt.ERROR = log.New(os.Stdout, "", 0)

	getClient()
	getDevices()
	if flagl || flagL {
		deviceInfo()
	}

	if flagcPresent {
		if flagc == "ALL" {
			for d, p := range deviceMap {
				if p["$state"] == "lost" {
					if clearDevice(d) {
						errors = true
					}
				}
			}
		} else {
			errors = clearDevice(flagc)
		}
		if errors {
			os.Exit(1)
		}
	}
}
