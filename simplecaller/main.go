/*
 * This is a simple caller.  Calls the simple server
 */

package main

import (
	//"fmt"
	"log"
	"net"
)

func main() {
	var server string
	server = "192.168.1.13:1884"
	conn, err := net.Dial("tcp", server)
	if err != nil {
		log.Fatal("Cannot connect to server " + server)
	}
	log.Print("Connected.  yay!")
	conn.Close()
}
