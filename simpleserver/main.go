/*
 * This is a simple server.  Listens on port 1884 and logs what it gets
 */

package main

import (
	//"fmt"
	"log"
	"net"
	"time"
)

func main() {
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
	log.Print("Got a connection!  Closing it.")
	c.Close()
}
