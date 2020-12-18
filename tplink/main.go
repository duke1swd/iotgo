/*
 * This program is our first program to hack tp-link smart plugs.
 */

package main

import (
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
	homieMap map[string]homieDevice
)

func init() {
	flag.BoolVar(&debug, "d", false, "debugging")

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
		if debug {
			fmt.Printf("Got %d bytes from addr %v: %s\n", n, addr, string(buf))
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
		if debug {
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

		if debug {
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

		if debug {
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

// Get the system info back.  Should all come back in 2 seconds
func buildDeviceMap(backChannel chan map[string]interface{}) {
	var device homieDevice

	timer := time.NewTimer(time.Duration(2) * time.Second)

	for {
		select {
		case <-timer.C:
			return
		case gmap := <-backChannel:
			a, ok := gmap["alias"]
			if !ok {
				if debug {
					fmt.Printf("gmap has no alias\n")
				}
				continue
			}
			name, ok := a.(string)
			if !ok {
				if debug {
					fmt.Printf("gmap alias is not a string (!)\n")
				}
				continue
			}
			device.name = homieName(name)

			ad, ok := gmap["addr"]
			if !ok {
				if debug {
					fmt.Printf("gmap has no addr\n")
				}
				continue
			}
			device.addr, ok = ad.(net.Addr)
			if !ok {
				if debug {
					fmt.Printf("gmap addr is not of type net.Addr\n")
				}
				continue
			}

			d, ok := gmap["deviceId"]
			if !ok {
				if debug {
					fmt.Printf("gmap has no deviceId\n")
				}
				continue
			}
			device.uid, ok = d.(string)
			if !ok {
				if debug {
					fmt.Printf("gmap deviceId is not a string (!)\n")
				}
				continue
			}

			r, ok := gmap["relay_state"]
			if !ok {
				if debug {
					fmt.Printf("gmap has no relay_state\n")
				}
				continue
			}
			relay, ok := r.(float64)
			if !ok {
				if debug {
					fmt.Printf("gmap relay_state (%v) is not an int (!)\n", relay)
				}
				continue
			}
			switch relay {
			case 0:
				device.on = false
			case 1:
				device.on = true
			default:
				if debug {
					fmt.Printf("gmap relay_state(%s) is not \"0\" or \"1\"\n", relay)
				}
				continue
			}

			homieMap[device.name] = device
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

	backChannel := make(chan map[string]interface{}, 100)

	go listenerSysinfo(pc, backChannel)

	// Broadcast our query
	broadcast(pc, []byte("{\"system\":{\"get_sysinfo\":null},\"emeter\":{\"get_realtime\":null}}"))

	// Collect the resposes
	buildDeviceMap(backChannel)

	if debug {
		fmt.Printf("Found these devices\n")
		for k, _ := range homieMap {
			fmt.Printf("\t%s\n", k)
		}
	}
}
