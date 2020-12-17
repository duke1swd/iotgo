/*
 * This program is our first program to hack tp-link smart plugs.
 */

package main

import (
	//"encoding/json"
	"flag"
	"fmt"
	"net"
	"time"
)

var (
	debug bool
)

func init() {
	flag.BoolVar(&debug, "d", false, "debugging")

	flag.Parse()
}

func tpEncode(data []byte) {
	var k byte

	k = 171
	for i := 0; i < len(data); i++ {
		data[i] = data[i] ^ k
		k = data[i]
	}
}

func tpDecode(data []byte) {
	var k byte

	k = 171
	for i := 0; i < len(data); i++ {
		t := data[i] ^ k
		k = data[i]
		data[i] = t
	}
}

func printData(data []byte) {
	if !debug {
		return
	}

	for _, v := range data {
		fmt.Printf("%2x ", v)
	}
	fmt.Printf("\n")
}

func broadcast(pc net.PacketConn, data []byte) {
	addr, err := net.ResolveUDPAddr("udp", "192.168.2.255:9999")
	if err != nil {
		panic(err)
	}

	printData(data)
	tpEncode(data)
	printData(data)

	_, err = pc.WriteTo(data, addr)
	if err != nil {
		panic(err)
	}
}

func listener(pc net.PacketConn) {
	buf := make([]byte, 1024)
	for {
		n, addr, err := pc.ReadFrom(buf)
		if err != nil {
			continue
		}
		tpDecode(buf)
		if debug {
			fmt.Printf("Got %d bytes from addr %v: %s\n", n, addr, string(buf))
		}
	}
}

func main() {
	// Create a packet connection
	pc, err := net.ListenPacket("udp", "") // listen for UDP on unspecified port
	if err != nil {
		panic(err)
	}
	defer pc.Close()

	go listener(pc)

	broadcast(pc, []byte("{\"system\":{\"get_sysinfo\":null},\"emeter\":{\"get_realtime\":null}}"))
	time.Sleep(time.Minute)
}
