package main

import (
	"testing"
	"log"
)


/*
 * These are the known keys, and their order
	"Service": -1,
	"Location": -1,
	"IOTTime": 0,
	"Seqn": 1,
	"MsgNum": 2,
	"MsgVal": 3,
	"Human": 4,
 */

func TestLogFormatter(t *testing.T) {
	var td = map[string]string {
		"Service": "Test Service",
		"A Key": "T1 A Key",
		"Human": "T1 Human String",
		"Seqn": "T1 Seqn",
		"B Key": "T1 B Key",
	}
	
	expectedResult := "T1 Seqn,T1 Human String,T1 A Key,T1 B Key"

	result := msgFormat(td)

	if result != expectedResult {
		log.Printf("Expected: %s", expectedResult)
		log.Printf("Result:   %s", result)
		t.Fatal("Format Miscompare")
	}
}
