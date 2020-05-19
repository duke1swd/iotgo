/*
 * Get the dnsexit credentials
 */

package main

import (
	"os"
	"io/ioutil"
	"strings"
	"log"
)

const defaultFileName = "/etc/ipupdate.credentials"

func init() {
	filename := os.Getenv("CREDENTIALS")
	if len(filename) < 1 {
		filename = defaultFileName
	}
	credbytes, err := ioutil.ReadFile(filename)
	if err != nil {
		log.Fatalf("Cannot read file %s.  Err=%v", filename, err)
	}
	credstrings := strings.Split(string(credbytes), ",")
	if len(credstrings) != 2 {
		log.Fatalf("Credential file %s should have exactly one ','", filename)
	}
	login = strings.Trim(credstrings[0], " \n")
	password = strings.Trim(credstrings[1], " \n")
}
