/*
 * This is the infinite loop that monitors our Internet availability.
 */

package main

import (
	"context"
	"os"
	"strconv"
	"time"
)

type state int

const (
	stateBooting = iota
	stateNoInternet
	stateNoWiFi
	stateGood
)

var (
	currentState   state
	stateEntryTime time.Time
	stateCounter   int
	attemptCounter int
	pollInterval   int
)

func init() {
	var err error

	currentState = stateBooting
	stateEntryTime = time.Now()

	pollInterval = defaultPollInterval
	pollIntervalString := os.Getenv("POLLINTERVAL")
	if len(pollIntervalString) > 0 {
		pollInterval, err = strconv.Atoi(pollIntervalString)
		if err != nil || pollInterval < 1 {
			pollInterval = defaultPollInterval
		}
	}
}

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

		worked = myPublishNow(ctx, msg, int(timeInState/time.Second))
		if !worked {
			myPublishEventually(logContactFailed, 0)
		}

		// figure out the new state.  May be same as old state
		if worked {
			currentState = stateGood
		} else if contactRouter() {
			currentState = stateNoInternet
		} else {
			myPublishEventually(logNoRouter, 0)
			currentState = stateNoWiFi
		}

		// keep track of how long we've been in this state, both in terms of
		// loops and time
		if currentState != oldState {
			stateEntryTime = time.Now()
			stateCounter = 0
			attemptCounter = 0
			msg = 0
			switch currentState {
			case stateNoInternet:
				msg = logStateInternetDown
			case stateNoWiFi:
				msg = logStateWiFiDown
			case stateGood:
				msg = logStateInternetUp
			}
			myPublishEventually(msg, 0)
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
		time.Sleep(time.Duration(pollInterval) * time.Second)
	}
}
