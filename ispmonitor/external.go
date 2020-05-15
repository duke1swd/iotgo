package main

import (
	"context"
	"net/http"
)

/*
 * Contact the router
 */
 func contactRouter() bool {
	 resp, err := http.Get("http://192.168.1.1")
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
