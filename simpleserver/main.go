/*
 * This is a simple server.  Listens on port 1884 and logs what it gets
 */

package main

import (
	//"fmt"
	"log"
	"net"
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

func getLine(c net.Conn) (string, bool) {
	var s string

	buffer := make([]byte, 0, 1024)

	// Read timeout is 2 seconds from now
	for len(s) < 128 {
		err := c.SetReadDeadline(time.Now().Add(time.Second * 2))
		if err != nil {
			log.Print("Failed to set read timeout")
		}

		n, err := c.Read(buffer)
		if err != nil {
			log.Print("Read error or timeout")
			return "", false
		}
if n > 0 {
log.Print("got " + strconv.Itoa(n) + " bytes")
}
		for i := 0; i < n; i++ {
			if buffer[i] == '\n' {
				return s, true
			}
			s += string([]byte{buffer[i]})
		}
	}
	log.Print("no newline found");
	return "", false
}

func handleConnection(c net.Conn) {
	log.Print("Connection Established")

	s, ok := getLine(c)
	if ! ok {
		log.Print("read line failed")
	} else {
		log.Print("got ." + s + ".");
	}

	c.Close()
}
