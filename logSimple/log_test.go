package logSimple

import (
	"context"
	"testing"
)

const testLocation = "Home"
const testService = "logSimpleTest"

func TestLog(t *testing.T) {
	ctx := context.Background()

	LogInit(ctx, testLocation, testService)

	seqn := 0
	msgNum := 1
	msgVal := 1
	human := "Test Message The First"
	if ! Log(ctx, seqn, msgNum, msgVal, human) {
		t.Fatal("Log 1 Failed")
	}

	seqn = 1
	msgVal = 2
	human = "Test Message The Second"
	if ! Log(ctx, seqn, msgNum, msgVal, human) {
		t.Fatal("Log 2 Failed")
	}
}
