/*
 * This program pulls the homie stuff out of mosquitto and displays it.
 */

package main

import (
	"fmt"
	"log"
	"os"
	"time"
	"strings"
	"strconv"

	"github.com/eclipse/paho.mqtt.golang"
)

var (
	epoch time.Time
)

func init() {
	epoch, _ = time.Parse("2006-Jan-02 MST", "2018-Nov-01 EDT")
}

var f mqtt.MessageHandler = func(client mqtt.Client, msg mqtt.Message) {
	topic := msg.Topic()
	payload := string(msg.Payload())
	if strings.Contains(topic, "firmware") {
		payload = "(suppressed)"
	}
	if strings.Contains(topic, "time") && !strings.Contains(topic, "uptime") {
		t, err := strconv.ParseInt(payload, 10, 64)
		if err == nil {
			etime := epoch.Add(time.Duration(t) * time.Second)
			// if the payload appears to be a time, convert it to a sensible looking time
			payload += " (" +  etime.Format("Mon Jan 2 15:04:05 -0700 EST 2006") + ")"
		}
	}
	fmt.Printf("%s: %s\n", topic, payload)
}

func main() {
	//mqtt.DEBUG = log.New(os.Stdout, "", 0)
	mqtt.ERROR = log.New(os.Stdout, "", 0)
	opts := mqtt.NewClientOptions().AddBroker("tcp://127.0.0.1:1883").SetClientID("mqtt-flow")
	opts.SetKeepAlive(60 * time.Second)
	opts.SetDefaultPublishHandler(f)
	opts.SetPingTimeout(1 * time.Second)

	c := mqtt.NewClient(opts)
	if token := c.Connect(); token.Wait() && token.Error() != nil {
		panic(token.Error())
	}

	if token := c.Subscribe("#", 0, nil); token.Wait() && token.Error() != nil {
		fmt.Println(token.Error())
		os.Exit(1)
	}

	time.Sleep(600 * time.Second)

	if token := c.Unsubscribe("#"); token.Wait() && token.Error() != nil {
		fmt.Println(token.Error())
		os.Exit(1)
	}

	c.Disconnect(250)

	time.Sleep(1 * time.Second)
}
