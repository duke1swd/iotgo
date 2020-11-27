/*
 * This program manages firmware on HOMIE devices
 */

package main

import (
	"context"
	"crypto/md5"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/eclipse/paho.mqtt.golang"
)

var (
	flagu        bool
	flagl        bool
	flagD        bool
	flagF        bool
	flagd        string
	flagf        string
	flagdPresent bool
	flagfPresent bool
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

	flag.BoolVar(&flagu, "u", false, "upload firmware")
	flag.BoolVar(&flagl, "l", false, "list devices or device (with -d) or verify a firmware file (with -f)")
	flag.BoolVar(&flagD, "D", false, "debugging")
	flag.BoolVar(&flagF, "F", false, "clear our OTA crap and reset the device")
	flag.StringVar(&flagd, "d", "", "name of a device")
	flag.StringVar(&flagf, "f", "", "firmware file name")

	flag.Parse()
	flagdPresent = (flagd != "")
	flagfPresent = (flagf != "")

	errors := 0

	if flagF && !flagdPresent {
		fmt.Printf("-F (Force) requires -d <dev>\n")
		errors += 1
	}

	if flagu && !flagdPresent {
		fmt.Printf("-u (update) requires -d <device>\n")
		errors += 1
	}

	if flagu && !flagfPresent {
		fmt.Printf("-u (update) requires -f <firmware file>\n")
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
	info := []string{"name", "version", "checksum"}

	dmap, ok := deviceMap[device]
	if !ok {
		fmt.Printf("Device %s not found in mqtt database\n", device)
		return
	}

	fmt.Printf("%s:\n", device)
	for _, field := range info {
		v, ok := dmap["$fw/"+field]
		if !ok {
			fmt.Printf("\tFW %s is missing\n", field)
		} else {
			fmt.Printf("\tFW %s: %s\n", field, v)
		}
	}
	if v, ok := dmap["$state"]; ok {
		fmt.Printf("\tState: %s\n", v)
	}
}

func fileDigest(file string) string {
	f, err := os.Open(file)
	if err != nil {
		fmt.Printf("Cannot open file %s for reading\n", file)
		return ""
	}
	defer f.Close()

	h := md5.New()
	if _, err := io.Copy(h, f); err != nil {
		fmt.Printf("I/O error reading file %s\n", file)
		return ""
	}

	// convert the digest from an array of bytes to a hex string
	d := h.Sum(nil)
	s := ""
	for _, b := range d {
		t := strconv.FormatUint(uint64(b), 16)
		if len(t) < 2 {
			s += "0"
		}
		s += t
	}
	return s
}

func fileInfo(file string) {
	fmt.Printf("File %s:\n", file)
	digest := fileDigest(file)
	if digest != "" {
		fmt.Printf("\tDigest: %s\n", digest)
	}
}

/*
 * This is the message handler for the OTA status messages.  It drives the OTA state machine forward.
 */
var lastMessage string = ""
var f2 mqtt.MessageHandler = func(client mqtt.Client, msg mqtt.Message) {
	var status int

	message := string(msg.Payload())
	topic := msg.Topic()

	if flagD {
		fmt.Printf("OTA Status message: %s->%s\n", topic, message)
	}

	if strings.HasSuffix(topic, "$implementation/ota/status") {
		if message == lastMessage { // sometimes they repeat
			return
		}
		lastMessage = message

		// when we clear out the OTA status code 200, we'll get a null message.
		if message == "" {
			return
		}

		n, err := fmt.Sscanf(message, "%d", &status)
		if err != nil {
			fmt.Printf("got error scanning status: %v\n", err)
			fmt.Printf("status is .%s.\n", status)
			return
		}
		if n != 1 {
			fmt.Printf("Sscanf of status failed.  n=%d\n", n)
			return
		}

		if status == 200 {
			fmt.Printf("Firmware upload successful.  Awaiting device reboot.\n")
		} else if status == 202 {
			fmt.Printf("Firmware update accepted.\n")
		} else if status == 206 {
			// update on how things are progressing
			var bytes int
			var total int
			var discard int
			r, err := fmt.Sscanf(message, "%d %d/%d", &discard, &bytes, &total)
			if r != 3 || err != nil {
				fmt.Printf("\nUnknown status message:", message)
			} else {
				fmt.Printf("Status: %.1f%%\r", float32(bytes)/float32(total)*100.)
			}
		} else if status == 304 {
			// new firmware same as old
			fmt.Printf("Firmware is already up to date.\n")
			f2done(status)
		} else if status == 400 {
			fmt.Printf("Malformed firmware checksum.  Firmware rejected.\n")
			f2done(status)
		} else if status == 403 {
			fmt.Printf("Device OTA disabled.  Firmware rejected.\n")
			f2done(status)
		} else if status == 500 {
			fmt.Printf("Internal error.  Firmware rejected.\n")
			f2done(status)
		} else if status > 300 && status < 500 {
			fmt.Printf("\nUnknown OTA error: '%s'.  Aborting\n", message)
			f2done(status)
		} else {
			fmt.Printf("\nUnknown OTA error: '%s'.\n", message)
		}
	} else if strings.HasSuffix(topic, "$fw/checksum") {
		returnedChecksum = message
		f2done(0)
		return
	}
}

// Tell the world the OTA is finished
// status 0 == OK.
func f2done(status int) {
	otaChannel <- status
}

/*
 * This routine subscribes to the OTA status messages and begins the firmware upload
 * by publishing the firmware.
 */
func publishFirmware(digest string) {
	opts := mqtt.NewClientOptions().AddBroker("tcp://127.0.0.1:1883").SetClientID("fw-test")
	opts.SetKeepAlive(60 * time.Second)
	opts.SetDefaultPublishHandler(f2)
	opts.SetPingTimeout(1 * time.Second)

	// Create a client for this task
	c := mqtt.NewClient(opts)
	if token := c.Connect(); token.Wait() && token.Error() != nil {
		panic(token.Error())
	}

	defer c.Disconnect(250)

	// Subscribe to the OTA updates
	// The handler for these messages drives the OTA process forward
	subscription := "devices/" + flagd + "/$implementation/ota/status"

	if token1 := c.Subscribe(subscription, 0, nil); token1.Wait() && token1.Error() != nil {
		fmt.Println(token1.Error())
		os.Exit(1)
	}

	defer func() {
		if token1 := c.Unsubscribe(subscription); token1.Wait() && token1.Error() != nil {
			fmt.Println(token1.Error())
			os.Exit(1)
		}
	}()

	if flagD {
		fmt.Printf("Subscribed to %s\n", subscription)
	}

	// Subscribe to the FW stuff
	// We will use an update to the firmware checksum to validate the upload
	subscription = "devices/" + flagd + "/$fw/#"

	if token2 := c.Subscribe(subscription, 0, nil); token2.Wait() && token2.Error() != nil {
		fmt.Println(token2.Error())
		os.Exit(1)
	}

	defer func() {
		if token2 := c.Unsubscribe(subscription); token2.Wait() && token2.Error() != nil {
			fmt.Println(token2.Error())
			os.Exit(1)
		}
	}()

	if flagD {
		fmt.Printf("Subscribed to %s\n", subscription)
	}

	// After the subscriptions are set up, we expect to get one "complete" signal back from
	// the message handler.  This is because the checksum message is persistent.
	_ = <-otaChannel

	// publish the firmware.  If all goes well this is all that is required to get the update done.
	topic := "devices/" + flagd + "/$implementation/ota/firmware/" + digest
	if flagD {
		fmt.Printf("Publishing firmware to topic %s\n", topic)
	}

	firmwarePayload, err := ioutil.ReadFile(flagf)
	if err != nil {
		panic(err)
	}

	// qos = 0, retain = false
	publishToken := c.Publish(topic, 0, false, firmwarePayload)
	if publishToken.Wait() && publishToken.Error() != nil {
		panic(publishToken.Error())
	}
	if flagD {
		fmt.Printf("Firmware published\n")
	}

	// wait for OTA to finish, or die
	status := <-otaChannel
	if flagD {
		fmt.Printf("got status %d back from message handler\n", status)
	}

	// Note that if status is not zero an error has already been printed
	if status == 0 {
		if returnedChecksum == digest {
			fmt.Printf("\nDone\n")
		} else {
			fmt.Printf("\nUpload Failed.\n")
			fmt.Printf("Sent with digest:     %s\n", digest)
			fmt.Printf("Received with digest: %s\n", returnedChecksum)
		}
	}

	// clean up the status message
	topic = "devices/" + flagd + "/$implementation/ota/status"
	publishToken = c.Publish(topic, 0, true, "")
	if publishToken.Wait() && publishToken.Error() != nil {
		panic(publishToken.Error())
	}
}

/*
 * This routine cleans up the OTA status message
 */
func cleanUp() {
	opts := mqtt.NewClientOptions().AddBroker("tcp://127.0.0.1:1883").SetClientID("fw-test")
	opts.SetKeepAlive(60 * time.Second)
	opts.SetPingTimeout(1 * time.Second)

	// Create a client for this task
	c := mqtt.NewClient(opts)
	if token := c.Connect(); token.Wait() && token.Error() != nil {
		panic(token.Error())
	}

	defer c.Disconnect(250)

	// clean up the status message
	topic := "devices/" + flagd + "/$implementation/ota/status"
	publishToken := c.Publish(topic, 0, true, "")
	if publishToken.Wait() && publishToken.Error() != nil {
		panic(publishToken.Error())
	}
}

func updateMode() {
	getDevices()
	devInfo, ok := deviceMap[flagd]
	if !ok {
		fmt.Printf("Cannot find device %s in device map\n", flagd)
		return
	}

	digest := fileDigest(flagf)
	if digest == "" {
		fmt.Printf("Cannot get digest for firmware file %s\n", flagf)
		return
	}

	if devInfo["$fw/checksum"] == digest {
		fmt.Printf("Device %s already running firmware with this digest\n", flagd)
		return
	}

	if flagD {
		fmt.Printf("Device digest .%s.\n  File digest .%s.\n",
			devInfo["$fw/checksum"],
			digest)
	}

	if devInfo["$online"] == "true" {
		homieVersion = 2
	} else if devInfo["$state"] == "ready" {
		homieVersion = 3
	} else {
		fmt.Printf("Device %s is not online and/or ready.\n", flagd)
		return
	}

	if flagD {
		fmt.Printf("Device %s is online.\n", flagd)
		fmt.Printf("Homie version is %d.\n", homieVersion)
	}

	if homieVersion != 3 {
		fmt.Printf("Upgrade only supports Homie version 3 at the moment.  Device version is %d\n", homieVersion)
		return
	}

	status, ok := devInfo["$implementation/ota/status"]
	if ok {
		fmt.Printf("Device %s is showing OTA status \"%s\" before OTA starts\n",
			flagd,
			status)
		fmt.Printf("Use -F mode to clear this\n")
		return
	}
	if flagD {
		fmt.Printf("Device %s OTA status is clear\n", flagd)
	}

	publishFirmware(digest)
}

func main() {
	//mqtt.DEBUG = log.New(os.Stdout, "", 0)
	mqtt.ERROR = log.New(os.Stdout, "", 0)

	// multiple modes
	if flagl {
		if flagfPresent {
			fileInfo(flagf)
		} else if flagdPresent {
			getDevices()
			deviceInfo(flagd)
		} else {
			fmt.Printf("Found these devices:\n")
			getDevices()
			for k, _ := range deviceMap {
				deviceInfo(k)
			}
		}
	} else if flagu {
		updateMode()
	} else if flagF {
		if !flagdPresent {
			fmt.Printf("-F requires a device (-d)\n")
		} else {
			cleanUp()
		}
	} else {
		fmt.Printf("Only list (\"-l\"), cleanup (\"-F\"), and update (\"-u\") modes are presently implemented\n")
	}
}
