package main

import (
	"context"
	"testing"
)

func TestPublish1(t *testing.T) {
	ctx, cxf := context.WithCancel(context.Background())
	defer cxf()

	myPublishInit(ctx)
	if !myPublishNow(ctx, logMessage(0), 0) {
		t.Fatal("Immediate publish failed.  Credentials set?")
	}
}
