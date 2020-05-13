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
	"github.com/duke1swd/iotgo/logQueue"
)

var (
	oldNow int64
	seqn int
	epoch time.Time
)

func main() {
	var err error

	ctx, cxf := context.WithCancel(context.Background())
	defer cxf()

	epoch, err = time.Parse("2006-Jan-02 MST", "2018-Nov-01 EDT");
	if err != nil {
		log.Fatal("failed to get epoch. Err = %v", err);
	}


	err = logQueue.Start(ctx, publishDeferredMessage)
	if err != nil {
		log.Fatal("failed to start log queue. Err = %v", err);
	}

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
	myPublishNow(ctx, topic, 1, 0, "human readable version")
}

/*
  Log messages have these attributes.
   - A time the message was recorded, which may be signicantly before it was published.
   - A sequence number.  Messages published at the same time may have different sequence numbers.
   - A message number.  Basically, what event is being logged
   - An integer value that may contain relevant information about the logged event.
   - A string that is the human readable version of the message.
 */

 /*
  This routine publishes a log message directly.
     This routine generates "now" as the time.
     This routine generates the sequence number
  Returns true of it was able to publish.
  Uses a 10 second timeout on the publish.
 */
func myPublishNow(ctx context.Context, tpx * pubsub.Topic, msgNum, msgVal int, human string) (retval bool) {
	now := int64(time.Since(epoch) / time.Second)
	if now != oldNow {
		seqn = 0
		oldNow = now
	}
	retval = myPublish(ctx, tpx, now, seqn, msgNum, msgVal, human)
	seqn++
	return
}


/*
    This routine actually publishes messages, whether directly or delayed.
 */
func myPublish(ctx context.Context, tpx * pubsub.Topic, when int64, seqn, msgNum, msgVal int, human string) bool {
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

/*
    Publish a log message that got deferred until now.
 */
func publishDeferredMessage(ctx context.Context, t, s string) bool {
	return false
}
