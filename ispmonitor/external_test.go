package main

import (
	"testing"
	//"log"
)


// first test. The background thread should never call the sender
func TestContactRouter(t *testing.T) {
	ok := contactRouter()
	if !ok {
		t.Fatalf("Could not contact router")
	}
}
