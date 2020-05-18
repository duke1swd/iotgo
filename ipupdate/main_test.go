package main

import (
	"testing"
	//"net"
	"fmt"
)

func TestMyIPAddress(t *testing.T) {
	ip, ok := myIPAddress()
	if ok {
		ipS := ip.String()
		if ipS != "" {
			fmt.Printf("My address is %s\n", ipS)
		} else {
			t.Fatal("IP address to string failed")
		}
	} else {
		t.Fatal("No IP address")
	}
}
