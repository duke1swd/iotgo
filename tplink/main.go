/*
 * This program is our first program to hack tp-link smart plugs.
 */

package main

import (
	"encoding/binary"
	"encoding/json"
	"flag"
	"fmt"
	"net"
	"time"
)

type homieDevice struct {
	uid  string
	name string
	addr net.Addr
	on   bool
}

var (
	debug    bool
	debugV   bool
	homieMap map[string]homieDevice
)

func init() {
	flag.BoolVar(&debug, "d", false, "debugging")
	flag.BoolVar(&debugV, "D", false, "extreme debugging")

	flag.Parse()

	homieMap = make(map[string]homieDevice)
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
	if !debugV {
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

func listenerSysinfo(pc net.PacketConn, output chan map[string]interface{}) {
	var response interface{}

	buf := make([]byte, 1024)
	for {
		n, addr, err := pc.ReadFrom(buf)
		if err != nil {
			continue
		}
		buf = buf[:n] // throw away the rest of the buffer
		tpDecode(buf)
		if debugV {
			fmt.Printf("Got %d bytes from addr %v: %s\n", n, addr, string(buf))
		} else if debug {
			fmt.Printf("Got %d bytes from addr %v\n", n, addr)
		}

		// This long string of code peels away fluff and leaves us with the sysinfo object.
		response = nil
		if err := json.Unmarshal(buf, &response); err != nil {
			// not valid json
			if debug {
				fmt.Printf("Not valid json: %v\n", err)
			}
			continue
		}
		rmap, ok := response.(map[string]interface{})
		if !ok {
			if debug {
				fmt.Printf("response is not a map\n")
			}
			continue
		}
		if debugV {
			fmt.Printf("response keys found:\n")
			for k, _ := range rmap {
				fmt.Printf("\t%s\n", k)
			}
		}
		s, ok := rmap["system"]
		if !ok {
			if debug {
				fmt.Printf("No system entry in response json\n")
			}
			continue
		}

		smap, ok := s.(map[string]interface{})
		if !ok {
			if debug {
				fmt.Printf("System entry is not a map\n")
			}
			continue
		}

		if debugV {
			fmt.Printf("system keys found:\n")
			for k, _ := range smap {
				fmt.Printf("\t%s\n", k)
			}
		}

		g, ok := smap["get_sysinfo"]
		if !ok {
			if debug {
				fmt.Printf("No get_sysinfo entry in response json\n")
			}
			continue
		}

		gmap, ok := g.(map[string]interface{})
		if !ok {
			if debug {
				fmt.Printf("Get_sysinfo entry is not a map\n")
			}
			continue
		}

		if debugV {
			fmt.Printf("get_sysinfo keys found:\n")
			for k, _ := range gmap {
				fmt.Printf("\t%s\n", k)
			}
		}
		gmap["addr"] = addr
		output <- gmap
	}
}

// converts a plug's nickname into a valid homie name.
func homieName(alias string) string {
	return alias
}

// Convert the json stuff that came back from the tp-link to our homieDevice
func tp2homie(gmap map[string]interface{}) (homieDevice, bool) {

	var device homieDevice

	a, ok := gmap["alias"]
	if !ok {
		if debug {
			fmt.Printf("gmap has no alias\n")
		}
		return device, false
	}
	name, ok := a.(string)
	if !ok {
		if debug {
			fmt.Printf("gmap alias is not a string (!)\n")
		}
		return device, false
	}
	device.name = homieName(name)

	ad, ok := gmap["addr"]
	if !ok {
		if debug {
			fmt.Printf("gmap has no addr\n")
		}
		return device, false
	}
	device.addr, ok = ad.(net.Addr)
	if !ok {
		if debug {
			fmt.Printf("gmap addr is not of type net.Addr\n")
		}
		return device, false
	}

	d, ok := gmap["deviceId"]
	if !ok {
		if debug {
			fmt.Printf("gmap has no deviceId\n")
		}
		return device, false
	}
	device.uid, ok = d.(string)
	if !ok {
		if debug {
			fmt.Printf("gmap deviceId is not a string (!)\n")
		}
		return device, false
	}

	r, ok := gmap["relay_state"]
	if !ok {
		if debug {
			fmt.Printf("gmap has no relay_state\n")
		}
		return device, false
	}
	relay, ok := r.(float64)
	if !ok {
		if debug {
			fmt.Printf("gmap relay_state (%v) is not an float (!)\n", relay)
		}
		return device, false
	}
	switch int(relay) {
	case 0:
		device.on = false
	case 1:
		device.on = true
	default:
		if debug {
			fmt.Printf("gmap relay_state(%f) is not 0 or 1\n", relay)
		}
		return device, false
	}

	return device, true
}

// Process events and do things
func run(backChannel chan map[string]interface{}) {
	for {
		gmap := <-backChannel

		device, ok := tp2homie(gmap)
		if ok {
			homieMap[device.uid] = device
			if debug {
				fmt.Printf("Got device %s\n", device.name)
				fmt.Printf("\tRelay is On: %v\n", device.on)
				fmt.Printf("\tAddress: %s\n", device.addr.String())
			}
			_, ok := callTCP(device, "{\"system\":{\"set_relay_state\":{\"state\":1}}}")
			fmt.Printf("call returns %v\n", ok)
		}
	}
}

// Call and response to the device over TCP
func callTCP(device homieDevice, call string) (interface{}, bool) {
	var dialer net.Dialer
	dialer.Timeout = time.Duration(2) * time.Second // the device responds quickly or not at all

	conn, err := dialer.Dial("tcp", device.addr.String())
	if err != nil {
		if debug {
			fmt.Printf("dial out to device via tcp failed: %v\n", err)
		}
		return nil, false
	}
	defer conn.Close()

	// First, tell the device how long the message will be
	lengthBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(lengthBytes, uint32(len(call)))
	n, err := conn.Write(lengthBytes)
	if n != 4 {
		if debug {
			fmt.Printf("Wrote %d bytes rather than 4\n", n)
		}
		return nil, false
	}
	if err != nil {
		if debug {
			fmt.Printf("Error on writing length to TCP: %v\n", err)
		}
		return nil, false
	}

	// Now, write the message
	writeBuffer := []byte(call)
	tpEncode(writeBuffer)
	n, err = conn.Write(writeBuffer)
	if n != len(call) {
		if debug {
			fmt.Printf("Wrote %d bytes rather than %d\n", n, len(call))
		}
		return nil, false
	}
	if err != nil {
		if debug {
			fmt.Printf("Error on writing message to TCP: %v\n", err)
		}
		return nil, false
	}

	// Now, read back the length
	n, err = conn.Read(lengthBytes)
	if n != 4 {
		if debug {
			fmt.Printf("Read %d bytes rather than 4\n", n)
		}
		return nil, false
	}
	if err != nil {
		if debug {
			fmt.Printf("Error on reading length from TCP: %v\n", err)
		}
		return nil, false
	}
	lenToRead := int(binary.BigEndian.Uint32(lengthBytes))
	readBuffer := make([]byte, lenToRead)

	// Read the response message
	n, err = conn.Read(readBuffer)
	if n != lenToRead {
		if debug {
			fmt.Printf("Read %d bytes rather than %d\n", n, lenToRead)
		}
		return nil, false
	}
	if err != nil {
		if debug {
			fmt.Printf("Error on reading message from TCP: %v\n", err)
		}
		return nil, false
	}
	tpDecode(readBuffer)
	if debugV {
		fmt.Printf("Response is %s\n", string(readBuffer))
	}
	return nil, true
}

func main() {
	// Create a packet connection
	pc, err := net.ListenPacket("udp", "") // listen for UDP on unspecified port
	if err != nil {
		panic(err)
	}
	defer pc.Close()

	backChannel := make(chan map[string]interface{}, 100)

	go listenerSysinfo(pc, backChannel)

	// Broadcast our query
	broadcast(pc, []byte("{\"system\":{\"get_sysinfo\":null},\"emeter\":{\"get_realtime\":null}}"))

	// Collect the resposes
	run(backChannel)
}
