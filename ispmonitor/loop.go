package main

import (
	"context"
	"time"
)

// This loop runs forever
func mainLoop(ctx context.Context) {
	var (
		worked bool
		msg    logMessage
	)

	currentState = stateBooting
	stateCounter = 0

	for {
		timeInState := time.Since(stateEntryTime)

		oldState := currentState
		switch currentState {
		case stateBooting:
			msg = logHelloWorld
		case stateNoInternet:
			msg = logInternetDown
		case stateNoWiFi:
			msg = logWiFiDown
		case stateGood:
			msg = logLifeIsGood
		}

		worked = myPublishNow(ctx, int(msg), int(timeInState/time.Second), msg.String())

		// figure out the new state.  May be same as old state
		if worked {
			currentState = stateGood
		} else if contactRouter() {
			currentState = stateNoInternet
		} else {
			currentState = stateNoWiFi
		}

		// keep track of how long we've been in this state, both in terms of
		// loops and time
		if currentState != oldState {
			stateEntryTime = time.Now()
			stateCounter = 0
			attemptCounter = 0
		} else {
			stateCounter++
		}

		// If things are down, try to kick them
		if stateCounter == 2 || (stateCounter-2)%6 == 0 {
			attemptCounter++
			switch currentState {
			case stateNoWiFi:
				resetRouter(ctx)
			case stateNoInternet:
				resetModem(ctx)
			}
		}
	}
}
