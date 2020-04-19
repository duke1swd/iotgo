/*
 * This program monitors the health of the Internet.
 * Every 5 minutes it publishes an I'm OK message to google's pub/sub.
 * If that fail, then things are no OK.  Various log mesages result.
 *
 * The key for talking to google must be set up in the environment variables.
 */

package main

import (
	"context"
	//"fmt"
	"log"

	"cloud.google.com/go/pubsub"
)

func main() {
	ctx := context.Background()

	// Sets your Google Cloud Platform project ID.
	projectID := "iot-services-274518"

	// Creates a client.
	client, err := pubsub.NewClient(ctx, projectID)
	if err != nil {
		log.Fatalf("ISP Monitor: Failed to create client: %v", err)
	}

	// Sets the id for the new topic.
	topicID := "Logs"

	// Get the topic object
	topic := client.Topic(topicID)
	if topic == nil {
		log.Fatalf("ISP Monitor: Failed to get topic: %s", topicID)
	}
	defer topic.Stop()
	topic.PublishSettings = pubsub.DefaultPublishSettings
}
