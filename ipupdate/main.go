/*
 * This program finds out our external address
 * and updates a dynamic DNS provider
 */

package main

import (
	"context"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/duke1swd/iotgo/logSimple"
)

const defaultHost = "canyonranch.linkpc.net"
const defaultUpdateURL = "http://update.dnsexit.com/RemoteUpdate.sv"
const defaultLocation = "Home"
const defaultPollInterval = "5" // in minutes

var (
	proxies = [...]string{
		"ip.dnsexit.com",
		"ip2.dnsexit.com",
		"ip3.dnsexit.com",
	}
	login     string
	password  string
	host      string
	updateURL string
	location  string

	daemon         bool
	lastUpdateTime time.Time
	nextLoopTime   time.Time
	myIP           net.IP
	ipValid        bool
	ctx            context.Context
	pollInterval   time.Duration
	ipKnown        bool = false
)

func init() {
	host = os.Getenv("HOST")
	if len(host) < 1 {
		host = defaultHost
	}

	updateURL = os.Getenv("UPDATEURL")
	if len(updateURL) < 1 {
		updateURL = defaultUpdateURL
	}

	location = os.Getenv("LOCATION")
	if len(location) < 1 {
		location = defaultLocation
	}

	pollIntervalS := os.Getenv("POLLINTERVAL")
	if len(pollIntervalS) < 1 {
		pollIntervalS = defaultPollInterval
	}
	pollIntervalI, err := strconv.Atoi(pollIntervalS)
	if err != nil {
		pollIntervalI, _ = strconv.Atoi(defaultPollInterval)
	}
	pollInterval = time.Duration(pollIntervalI) * time.Minute

	flag.BoolVar(&daemon, "daemon", false, "Run forever")
}

func main() {
	var ok bool

	flag.Parse()

	if daemon {
		ctx = context.Background()

		myIP, ok = myIPAddress()
		if !ok {
			log.Println("IP Update: Cannot get IP address")
			daemonExit()
		}
		ipKnown = true
		lastUpdateTime = time.Now()

		ok = postIP(myIP)
		if !ok {
			log.Println("IP Update: Initial post failed")
			daemonExit()
		}

		log.Println("IP Update Daemon started.  IP = ", myIP.String())
		logSimple.LogInit(ctx, location, "IPUpdate")
		logSimple.Log(ctx, 0, 1, 0, "Startup. IP Address: "+myIP.String())
		ipValid = true
		nextLoopTime = lastUpdateTime.Add(pollInterval)
		updateLoop()
	} else {
		myIP, ok = myIPAddress()
		if ok {
			fmt.Printf("My IP address is %s\n", myIP.String())
		} else {
			fmt.Println("No IP address")
		}

		ok = postIP(myIP)
		if ok {
			fmt.Println("Successfully Posted")
		} else {
			fmt.Println("Failed to post")
		}
	}
}

func updateLoop() {
	for {
		var (
			newIP   net.IP
			howLong int
		)

		time.Sleep(time.Until(nextLoopTime))
		nextLoopTime = nextLoopTime.Add(pollInterval)

		newIP, ok := myIPAddress()
		if !ok {
			if ipValid {
				ipValid = false
				lastUpdateTime = time.Now()
			}
			howLong = int(time.Since(lastUpdateTime) / time.Second)
			logSimple.Log(ctx, 0, 2, howLong, "Failed to get IP address")
			continue
		}

		if !ipValid {
			ipValid = true
			lastUpdateTime = time.Now()
		} else if newIP.Equal(myIP) {
			logSimple.Log(ctx, 0, 0, howLong, "No IP Address Change")
			continue
		}

		myIP = newIP
		howLong = int(time.Since(lastUpdateTime) / time.Second)
		logSimple.Log(ctx, 0, 3, 0, "New IP address: "+myIP.String())
		lastUpdateTime = time.Now()
	}
}

// if we are a daemon, do not exit to fast.  Give some time for the issue to resolve
// before we are restarted
func daemonExit() {
	time.Sleep(time.Duration(60) * time.Second)
	os.Exit(1)
}

func myIPAddress() (net.IP, bool) {
	for _, s := range proxies {
		resp, err := http.Get("http://" + s)
		if err != nil {
			continue
		}

		body, err := ioutil.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			continue
		}
		bodyS := strings.Trim(string(body), " ")

		myIP := net.ParseIP(bodyS)
		if myIP == nil {
			continue
		}
		return myIP, true
	}
	return net.ParseIP("127.0.0.1"), false
}

func postIP(ip net.IP) bool {
	postURL := updateURL +
		"?login=" + login + "&password=" + password + "&host=" + host +
		"&myip=" + ip.String()

	resp, err := http.Get(postURL)
	if err != nil {
		log.Printf("Fail on GET to post IP address.  Err=%v", err)
		return false
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		log.Printf("Bad Response to GET on post IP address.  Code = %d", resp.StatusCode)
		return false
	}

	return true
}
