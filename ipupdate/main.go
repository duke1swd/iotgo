/*
 * This program finds out our external address
 * and updates a dynamic DNS provider
 */

package main

import (
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"strings"
)

var (
	proxies = [...]string{
		"ip.dnsexit.com",
		"ip2.dnsexit.com",
		"ip3.dnsexit.com",
	}
)

func main() {
	myIP, ok := myIPAddress()
	if ok {
		fmt.Printf("My IP address is %s\n", myIP.String())
	} else {
		fmt.Println("No IP address")
	}
}

func myIPAddress() (net.IP, bool) {
	for _, s := range proxies {
		fmt.Printf("Server: %s\n", s)
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

		fmt.Printf("\tGot %s\n", bodyS)
		myIP := net.ParseIP(bodyS)
		if myIP == nil {
			continue
		}
		return myIP, true
	}
	return net.ParseIP("127.0.0.1"), false
}
