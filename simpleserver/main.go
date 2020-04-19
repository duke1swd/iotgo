/*
 * This is a simple server.  Listens on port 1884 and logs what it gets
 */

package main

import (
	"fmt"
	"log"
	"net"
	"bufio"
	"time"
	"strconv"
	"strings"
	"unicode"
)

var epoch time.Time
var debug bool = true

func main() {
	epoch, err := time.Parse("2006-Jan-02 MST", "2018-Nov-01 EDT")
	if err != nil {
		log.Fatal("Startup: failed to get epoch")
	} else {
		duration := int64(time.Since(epoch) / time.Second)
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
	defer c.Close()

	if debug {
		fmt.Print("Connection Established")
	}

	reader := bufio.NewReader(c)
	err := c.SetReadDeadline(time.Now().Add(time.Second * 2))
	if err != nil {
		log.Print("Failed to set read timeout")
	}

	s, err := reader.ReadBytes('\n')
	if err != nil {
		log.Print("readBytes failed")
		log.Print(err)
		return
	}

	// Trim off the trailing newline (and other cruft, if present)
	line := strings.TrimFunc(string(s), unicode.IsSpace)

	if debug {
		fmt.Println("Got: " + line)
	}

	fields := strings.Split(line, ",")
	if debug {
		for i, f := range fields {
			fmt.Printf("%2d: %s\n", i, f)
		}
	}

	if len(fields) != 4 {
		// badly formed line
		returnValue(c, 0, 0)
		return
	}

	service := fields[0]
	location := fields[1]
	device := fields[2]
	password := fields[3]

	if password != "314159" {
		// bad password.  Really just an anti DDOS measure
		returnValue(c, 0, 0)
		return
	}

	log.Print(fmt.Sprintf("Ping from %s.%s.%s", service, location, device))
	epoch, _ := time.Parse("2006-Jan-02 MST", "2018-Nov-01 EDT")
	returnValue(c, 9, int64(time.Since(epoch) / time.Second))
}

// Format response and send it
func returnValue(c net.Conn, code int, value int64) {
	var msg string

	writer := bufio.NewWriter(c)

	if code == 0 {
		msg = "0\n"
	} else {
		msg = fmt.Sprintf("%d,%d\n", code, value)
	}

	if debug {
		fmt.Print("Responding with: " + msg)
	}

	writer.WriteString(msg)
	writer.Flush()
}
