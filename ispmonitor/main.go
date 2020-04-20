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
	"time"
	"strconv"

	"cloud.google.com/go/pubsub"
)

func main() {
	ctx := context.Background()

	// This is the IOT Services project
	projectID := "iot-services-274518"

	// Creates a client.
	client, err := pubsub.NewClient(ctx, projectID)
	if err != nil {
		log.Fatalf("ISP Monitor: Failed to create client: %v", err)
	}

	// Log messages go to topic Logs. 
	topicID := "Logs"

	// Get a pointer to the topic object
	topic := client.Topic(topicID)
	if topic == nil {
		log.Fatalf("ISP Monitor: Failed to get topic: %s", topicID)
	}
	defer topic.Stop()

	// request to send messages immediately
	topic.PublishSettings.CountThreshold = 1

	// Publish a sample message
	myPublishNow(ctx, topic, 1, 0)
}

/*
  This routine publishes a log message.
  Log messages have 3 attributes.
   - A time the message was recorded, which may be signicantly before it was published
   - A message number.  Basically, what event is being logged
   - An integer value that may contain relevant information about the logged event.

  Returns true of it was able to publish.
  Uses a 10 second timeout on the publish.
 */
func myPublishNow(ctx context.Context, tpx * pubsub.Topic, msgNum, msgVal int) bool {
	epoch, err := time.Parse("2006-Jan-02 MST", "2018-Nov-01 EDT");
	if err != nil {
		log.Fatal("failed to get epoch");
	}
	now := int64(time.Since(epoch) / time.Second)
	return myPublish(ctx, tpx, now, msgNum, msgVal)
}


func myPublish(ctx context.Context, tpx * pubsub.Topic, when int64, msgNum, msgVal int) bool {
	var myMsg pubsub.Message

	myMsg.Attributes = make(map[string]string)
	myMsg.Attributes["IOTTime"] = strconv.FormatInt(when, 10)
	myMsg.Attributes["MsgNum"] = strconv.Itoa(msgNum)
	myMsg.Attributes["MsgVal"] = strconv.Itoa(msgVal)

	ctxd, cancelFn := context.WithDeadline(ctx, time.Now().Add(10 * time.Second))
	defer cancelFn()

	result := tpx.Publish(ctxd, &myMsg)
	_, err := result.Get(ctxd)	
	if err != nil {
		log.Printf("publish get result returns error: %v", err)
		return false
	}

	return true
}
