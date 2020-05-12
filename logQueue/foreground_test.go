package logQueue

import (
	"testing"
	"time"
	"context"
	"strings"
	"fmt"
	"log"
)

var (
	blocked bool
	nblocked int
)

// write a log record and see that it comes back out
func TestLogWrite1(t *testing.T) {

	debugMode = true

	linkchan = make(chan string)
	defer close(linkchan)

	c, cxf := context.WithTimeout(context.Background(), 12 * time.Second)
	defer cxf()
	Start(c, mySender1)
	myMessage := "Test Message 1"
	err := Log(myMessage)
	if err != nil {
		t.Fatalf("Logging failed err = %v", err)
	}
	messages := 1

	for {
		select {
		case m := <- linkchan:
			if messages != 1 {
				t.Fatalf("Got unexpected or duplicate message in test log write 1: %s", m)
			}
			// did we get what we sent?
			mFields := strings.SplitN(m, " ", 2)
			if len(mFields) != 2 {
				t.Fatalf("Recieved message badly formatted: %s", m)
			}
			if mFields[1] != myMessage {
				t.Fatalf("Sent .%s. got .%s.", myMessage, mFields[1])
			}
			messages -= 1
		case <- c.Done():
			if messages != 0 {
				t.Fatalf("Message never received in test log write 1")
			}
			return
		}
	}
}

const nMess1 = 100
const nMess2 = 10

// write a bunch of log records
func TestLogWrite2(t *testing.T) {

	debugMode = true

	linkchan = make(chan string)
	defer close(linkchan)

	c, cxf := context.WithTimeout(context.Background(), 12 * time.Second)
	defer cxf()
	Start(c, mySender1)
	for i := 0; i < nMess1; i++ {
		err := Log(fmt.Sprintf("Test Message %d", i))
		if err != nil {
			t.Fatalf("Logging failed on message %d, err = %v", i, err)
		}
	}
	messages := nMess1

	for {
		select {
		case <- linkchan:
			if messages == 0 {
				t.Fatalf("Got too many messages")
			}
			messages -= 1
		case <- c.Done():
			if messages != 0 {
				t.Fatalf("Timed out waiting for %d more messages", messages)
			}
			return
		}
	}

	if clean(true) {
		t.Fatalf("Found files in log directory at end of test")
	}
}

// test failure retry
func TestLogWrite3(t *testing.T) {

	debugMode = true
	blocked = true
	nblocked = 0

	linkchan = make(chan string)
	defer close(linkchan)

	c, cxf := context.WithTimeout(context.Background(), 40 * time.Second)
	cT, _ := context.WithTimeout(c, 15 * time.Second)
	defer cxf()
	Start(c, mySender2)
	for i := 0; i < nMess2; i++ {
		err := Log(fmt.Sprintf("Test Message %d", i))
		if err != nil {
			t.Fatalf("Logging failed on message %d, err = %v", i, err)
		}
	}
	messages := nMess2

    testLoop:
	for {
		select {
		case <- linkchan:
			if messages == 0 {
				t.Fatalf("Got too many messages")
			}
			if blocked {
				t.Fatalf("Got message while blocked")
			}
			messages -= 1
		case <- cT.Done():
			blocked = false
		case <- c.Done():
			if messages != 0 {
				t.Fatalf("Timed out waiting for %d more messages", messages)
			}
			break testLoop
		}
	}
	if nblocked <= 0 {
		t.Fatalf("No messages sent during blocked period")
	}

	if nblocked > 4 {
		t.Fatalf("Too many (%d) messages received during blocked period", nblocked)
	}

	if clean(true) {
		t.Fatalf("Found files in log directory at end of test")
	}
}

func mySender2(t, s string, c context.Context) bool {
	if blocked {
		log.Printf("Sender called on %s %s.  Blocked", t, s)
		nblocked++
		return false
	}

	log.Printf("Sender called on %s %s", t, s)
	linkchan <- t + " " + s
	return true
}
