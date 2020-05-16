package main

import (
	"testing"
	//"log"
	"os"
)

// first test. The background thread should never call the sender
func TestContactRouter(t *testing.T) {
	ok := contactRouter()
	if !ok {
		t.Fatalf("Could not contact router")
	}
	err := os.Setenv("ROUTER", "8.8.8.8")
	if err != nil {
		t.Fatalf("Could not setenv")
	}
	ok = contactRouter()
	if ok {
		t.Fatalf("Contacted non-existent router")
	}
}
