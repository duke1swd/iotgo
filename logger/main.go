/*
 * This program subscribes to log messages at google
 * and puts them into a flat file.
 * The log messages are in a CSV format, begining with a timestamp and ending with the human readable version.
 */

package main

import (
        "context"
        "fmt"
        "io"
        "sync"

        "cloud.google.com/go/pubsub"
)

const defaultSubID = "Logger"

var (
	subID string
        mu sync.Mutex
)

func init() {
	subID = os.Getenv("SUBID")
	if len(subID) < 1 {
		subID = defaultSubID
	}
}

func main() {

        ctx, cfx := context.WithCancel(context.Background())
	defer cfx()

        client, err := pubsub.NewClient(ctx, projectID)
        if err != nil {
		log.Fatalf("ISP Monitor: Failed to create client: %v", err)
        }

        sub := client.Subscription(subID)
        err = sub.Receive(cctx, processor)
        if err != nil {
                return fmt.Errorf("Receive: %v", err)
        }

	// wait forever
	for {
		<- ctx.Done()
	}
}

func processor(ctx context.Context, msg *pubsub.Message) {
	msg.Ack()
	mu.Lock()
	defer mu.Unlock()
	//write the message to the log file
}
