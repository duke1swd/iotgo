package main

import (
	"context"
	"net/http"
	"os"
)

/*
 * Contact the router
 */
func contactRouter() bool {
	router := os.Getenv("ROUTER")
	if len(router) < 1 {
		router = defaultRouter
	}
	resp, err := http.Get("http://" + router)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	return true
}

/*
 * Hardware reset logic
 */

func resetRouter(ctx context.Context) {
	// we don't actually do anything yet.  Just log it and move on
	myPublishEventually(logWiFiReset, attemptCounter)
}

func resetModem(ctx context.Context) {
	// we don't actually do anything yet.  Just log it and move on
	myPublishEventually(logModemReset, attemptCounter)
}
