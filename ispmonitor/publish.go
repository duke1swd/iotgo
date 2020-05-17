package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"cloud.google.com/go/pubsub"
	"github.com/duke1swd/iotgo/logQueue"
)

const publishDeadline = 30 // timeout on publishing, in seconds

/*
  Log messages have these attributes.
   - A time the message was recorded, which may be signicantly before it was published.
   - A sequence number.  Messages published at the same time may have different sequence numbers.
   - A message number.  Basically, what event is being logged
   - An integer value that may contain relevant information about the logged event.
   - A string that is the human readable version of the message.
*/

var (
	topic    *pubsub.Topic
	location string
)

func init() {
	var err error

	location = os.Getenv("LOCATION")
	if len(location) < 1 {
		location = defaultLocation
	}

	epoch, err = time.Parse("2006-Jan-02 MST", "2018-Nov-01 EDT")
	if err != nil {
		log.Fatalf("failed to get epoch. Err = %v", err)
	}
}

func myPublishInit(ctx context.Context) {

	projectID := os.Getenv("PROJECTID")
	if len(projectID) < 1 {
		projectID = defaultProjectID
	}

	// Creates a client.
	client, err := pubsub.NewClient(ctx, projectID)
	if err != nil {
		log.Fatalf("ISP Monitor: Failed to create client: %v", err)
	}

	// Get a pointer to the topic object
	topic = client.Topic(topicID)
	if topic == nil {
		log.Fatalf("ISP Monitor: Failed to get topic: %s", topicID)
	}

	// request to send messages immediately
	topic.PublishSettings.CountThreshold = 1
}

/*
  This routine publishes a log message directly.
     This routine generates "now" as the time.
     This routine generates the sequence number
  Returns true of it was able to publish.
  Uses a 10 second timeout on the publish.
*/
func myPublishNow(ctx context.Context, msgNum, msgVal int, human string) (retval bool) {
	now := int64(time.Since(epoch) / time.Second)
	if now != oldNow {
		seqn = 0
		oldNow = now
	}
	retval = myPublish(ctx, now, seqn, msgNum, msgVal, fmt.Sprintf(human, msgVal))
	seqn++
	return
}

/*
 This routine sees to it that a log message gets published, eventually.
*/
func myPublishEventually(msgNum logMessage, msgVal int) {
	human := fmt.Sprintf(msgNum.String(), msgVal)
	logQueue.Log(fmt.Sprintf("%d,%d,%s", msgNum, msgVal, fmt.Sprintf(human, msgVal)))
}

/*
   This routine actually publishes messages, whether directly or delayed.
*/
func myPublish(ctx context.Context, when int64, seqn, msgNum, msgVal int, human string) bool {
	var myMsg pubsub.Message

	myMsg.Attributes = make(map[string]string)
	myMsg.Attributes["Service"] = service
	myMsg.Attributes["Location"] = location
	myMsg.Attributes["IOTTime"] = strconv.FormatInt(when, 10)
	myMsg.Attributes["Seqn"] = strconv.Itoa(seqn)
	myMsg.Attributes["MsgNum"] = strconv.Itoa(msgNum)
	myMsg.Attributes["MsgVal"] = strconv.Itoa(msgVal)
	myMsg.Attributes["Human"] = human

	ctxd, cancelFn := context.WithDeadline(ctx, time.Now().Add(publishDeadline*time.Second))
	defer cancelFn()

	result := topic.Publish(ctxd, &myMsg)
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

	// convert the file name into its pieces
	f := strings.SplitN(t, "_", 2)
	when, _ := strconv.ParseInt(f[0], 10, 64)

	k, _ := strconv.ParseInt(f[1], 10, 32)
	seqn := int(k)

	// convert the message number into its pieces
	f = strings.SplitN(s, ",", 3)

	k, _ = strconv.ParseInt(f[0], 10, 32)
	msgNum := int(k)

	k, _ = strconv.ParseInt(f[1], 10, 32)
	msgVal := int(k)

	human := f[2]

	return myPublish(ctx, when, seqn, msgNum, msgVal, human)
}
