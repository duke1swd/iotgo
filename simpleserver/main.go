/*
 * This is a simple server.  Listens on port 1884 and logs what it gets
 */

package main

import (
	//"fmt"
	"log"
	"net"
	"bufio"
	"time"
	"strconv"
)

var epoch time.Time

func main() {
	epoch, err := time.Parse("2006-Jan-02 MST", "2018-Nov-01 EDT")
	if err != nil {
		log.Print("Startup: failed to get epoch")
	} else {
		duration := int64(time.Since(epoch)) / 1000000000
		log.Print("Startup: Time since the epoch is " + strconv.FormatInt(duration, 10))
	}

	for {
		ln, err := net.Listen("tcp", ":1884")
		if err != nil {
			log.Print("Error listening on socket 1884")
			time.Sleep(10 * time.Second)
			continue
		}
		for {
			conn, err := ln.Accept()
			if err != nil {
				log.Print("Error accepting a connection. Closing listener and retrying")
				ln.Close()
				break
			}
			go handleConnection(conn)
		}
	}
}

func handleConnection(c net.Conn) {
	log.Print("Connection Established")

	reader := bufio.NewReader(c)
	err := c.SetReadDeadline(time.Now().Add(time.Second * 2))
	if err != nil {
		log.Print("Failed to set read timeout")
	}

	s, err := reader.ReadBytes('\n')
	line := string(s)

	if err != nil {
		log.Print("read line failed")
		log.Fatal(err)
	} else {
		log.Print("got ." + line + ".");
	}

	c.Close()
}
