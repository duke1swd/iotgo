package logQueue

import (
	"testing"
	"time"
	"context"
	"strings"
)

// write a log record and see that it comes back out
func TestLogWrite1(t *testing.T) {

	debugMode = true

	linkchan = make(chan string)
	defer close(linkchan)

	c, cxf := context.WithTimeout(context.Background(), 22 * time.Second)
	defer cxf()
	Start(c, mySender1);
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
