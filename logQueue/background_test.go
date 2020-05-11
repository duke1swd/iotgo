package logQueue

import (
	"testing"
	"time"
	"context"
)

var linkchan chan string

// first test. The background thread should never call the sender
func TestBackgroundLogThread1(t *testing.T) {

	linkchan = make(chan string)
	defer close(linkchan)

	c, cxf := context.WithTimeout(context.Background(), 30 * time.Second)
	defer cxf()

	backgroundLogThread(c, mySender1);
	select {
	case m := <- linkchan:
		t.Fatalf("Got unexpected message in test1: %s", m)
	case <- c.Done():
	}
}

func mySender1(t, s string, c context.Context) bool {
	linkchan <- t + " " + s
	return true
}
