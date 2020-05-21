package logSimple

import (
	"context"
	"log"
	"os"
	"strconv"
	"time"

	"cloud.google.com/go/pubsub"
)

const defaultProjectID = "iot-services-274518" // This is the IOT Services project
const topicID = "Logs"                         // Log messages go to topic Logs.

/*
  Log messages have these attributes.
   - A time the message was recorded, which may be signicantly before it was published.
   - A sequence number.  Messages published at the same time may have different sequence numbers.
   - A message number.  Basically, what event is being logged
   - An integer value that may contain relevant information about the logged event.
   - A string that is the human readable version of the message.
*/

var (
	topic     *pubsub.Topic
	location  string
	service   string
	projectID string
	epoch     time.Time
)

// we need a context, a location, and a service
func LogInit(ctx context.Context, l, s string) {
	var err error

	location = l
	service = s

	projectID := os.Getenv("PROJECTID")
	if len(projectID) < 1 {
		projectID = defaultProjectID
	}

	epoch, err = time.Parse("2006-Jan-02 MST", "2018-Nov-01 EDT")
	if err != nil {
		log.Fatalf("logSimple: Failed to get epoch. Err = %v", err)
	}

	// Creates a client.
	log.Printf("ProjectID = %s\n", projectID)
	client, err := pubsub.NewClient(ctx, projectID)
	if err != nil {
		log.Fatalf("logSimple: Failed to create client: %v", err)
	}

	// Get a pointer to the topic object
	topic = client.Topic(topicID)
	if topic == nil {
		log.Fatalf("logSimple: Failed to get topic: %s", topicID)
	}

	// request to send messages immediately
	topic.PublishSettings.CountThreshold = 1
}

func Log(ctx context.Context, seqn, msgNum, msgVal int, human string) bool {
	var myMsg pubsub.Message

	when := int64(time.Since(epoch) / time.Second)

	myMsg.Attributes = make(map[string]string)
	myMsg.Attributes["Service"] = service
	myMsg.Attributes["Location"] = location
	myMsg.Attributes["IOTTime"] = strconv.FormatInt(when, 10)
	myMsg.Attributes["Seqn"] = strconv.Itoa(seqn)
	myMsg.Attributes["MsgNum"] = strconv.Itoa(msgNum)
	myMsg.Attributes["MsgVal"] = strconv.Itoa(msgVal)
	myMsg.Attributes["Human"] = human

	result := topic.Publish(ctx, &myMsg)
	_, err := result.Get(ctx)
	if err != nil {
		log.Printf("logSimple: publish get result returns error: %v", err)
		return false
	}

	return true
}
