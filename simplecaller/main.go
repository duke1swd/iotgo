/*
 * This is a simple caller.  Calls the simple server
 */

package main

import (
	//"fmt"
	"log"
	"net"
	"bufio"
)

func main() {
	var server string
	server = "192.168.1.13:1884"
	conn, err := net.Dial("tcp", server)
	if err != nil {
		log.Fatal("Cannot connect to server " + server)
	}
	log.Print("Connected.  yay!")
	writer := bufio.NewWriter(conn)
	writer.WriteString("Hello World.\n");
	conn.Close()
}
