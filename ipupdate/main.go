/*
 * This program finds out our external address
 * and updates a dynamic DNS provider
 */

package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
)

const defaultLogin = "duke1swd"
const defaultPassword = "mMkf7E;TBzQJd7E^-H3G"
const defaultHost = "canyonranch.linkpc.net"
const defaultUpdateURL = "http://update.dnsexit.com/RemoteUpdate.sv"

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
)

func init() {
	login = os.Getenv("LOGIN")
	if len(login) < 1 {
		login = defaultLogin
	}

	password = os.Getenv("PASSWORD")
	if len(password) < 1 {
		password = defaultPassword
	}

	host = os.Getenv("HOST")
	if len(host) < 1 {
		host = defaultHost
	}

	updateURL = os.Getenv("UPDATEURL")
	if len(updateURL) < 1 {
		updateURL = defaultUpdateURL
	}
}

func main() {
	myIP, ok := myIPAddress()
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
