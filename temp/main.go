/*
 * This program prints out the current temperature
 */

package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/eclipse/paho.mqtt.golang"
)


var (
	mqttClient          mqtt.Client
	timeoutContext      context.Context
	timeoutCancel context.CancelFunc
)

var f1 mqtt.MessageHandler = func(client mqtt.Client, msg mqtt.Message) {
	//topic := msg.Topic()
	payload := string(msg.Payload())

	// print out the current temperature
	fmt.Println(payload)
	timeoutCancel()
}

func getClient() {
	opts := mqtt.NewClientOptions().AddBroker("tcp://192.168.1.13:1883").SetClientID("temp")
	opts.SetKeepAlive(60 * time.Second)
	opts.SetDefaultPublishHandler(f1)
	opts.SetPingTimeout(1 * time.Second)

	c := mqtt.NewClient(opts)
	if token := c.Connect(); token.Wait() && token.Error() != nil {
		panic(token.Error())
	}
	mqttClient = c
}

func subscribeAndRun() {
	c := mqttClient

	subscription := "environment/outdoor-temp"

	d, _ := time.ParseDuration("2s")
	timeoutContext, timeoutCancel = context.WithTimeout(context.Background(), d)

	if token := c.Subscribe(subscription, 0, nil); token.Wait() && token.Error() != nil {
		fmt.Println(token.Error())
		os.Exit(1)
	}

	select {
		case <- timeoutContext.Done():	// Wait for the context to timeout or be cancelled
	}

	if token := c.Unsubscribe(subscription); token.Wait() && token.Error() != nil {
		fmt.Println(token.Error())
		os.Exit(1)
	}
}

func main() {
	//mqtt.DEBUG = log.New(os.Stdout, "", 0)
	mqtt.ERROR = log.New(os.Stdout, "", 0)

	getClient()
	subscribeAndRun()
}
