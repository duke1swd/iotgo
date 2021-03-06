/*
 * This program subscribes to log messages at google
 * and puts them into a flat file.
 * The log messages are in a CSV format, begining with a timestamp and ending with the human readable version.
 */

package main

import (
	"context"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"cloud.google.com/go/pubsub"
)

const defaultSubID = "Logger"
const defaultProjectID = "iot-services-274518" // This is the IOT Services project
const defaultLogDirectory = "/var/log"
const defaultFiltering = true

var (
	subID          string
	projectID      string
	logDirectory   string
	filtering      bool
	mu             sync.Mutex
	epoch          time.Time
	repeatFilter   map[string]bool  = make(map[string]bool)
	lastMessagePos map[string]int64 = make(map[string]int64)
	lastDay        map[string]int   = make(map[string]int)
)

func init() {
	var err error

	subID = os.Getenv("SUBID")
	if len(subID) < 1 {
		subID = defaultSubID
	}

	projectID = os.Getenv("PROJECTID")
	if len(projectID) < 1 {
		projectID = defaultProjectID
	}

	logDirectory = os.Getenv("LOGDIR")
	if len(logDirectory) < 1 {
		logDirectory = defaultLogDirectory
	}

	filtering = defaultFiltering
	fs := os.Getenv("FILTERING")
	switch strings.ToLower(fs) {
	case "false":
		filtering = false
	case "f":
		filtering = false
	case "true":
		filtering = true
	case "t":
		filtering = true
	}

	epoch, err = time.Parse("2006-Jan-02 MST", "2018-Nov-01 EDT")
	if err != nil {
		log.Fatalf("Logger: Failed to get epoch. Err = %v", err)
	}
}

func main() {

	ctx, cfx := context.WithCancel(context.Background())
	defer cfx()

	client, err := pubsub.NewClient(ctx, projectID)
	if err != nil {
		log.Fatalf("Logger: Failed to create client: %v", err)
	}

	sub := client.Subscription(subID)
	err = sub.Receive(ctx, processor)
	if err != nil {
		log.Fatalf("Logger: Receive returns error: %v", err)
	}

	// wait forever
	log.Println("Logger: Entering Daemon Loop")
	for {
		<-ctx.Done()
	}
}

func processor(ctx context.Context, msg *pubsub.Message) {
	msg.Ack()

	// format the time stamp
	iottime, err := strconv.Atoi(msg.Attributes["IOTTime"])
	if err != nil {
		iottime = 0
	}
	stampTime := epoch.Add(time.Duration(iottime) * time.Second)
	formattedMsg := stampTime.Format("Mon Jan 2 15:04:05 2006") + ", "

	// format the attributes for the log file
	formattedMsg += msgFormat(msg.Attributes) + "\n"

	// make the log file name.  Should be for the form "Location_Service"
	logFileName, ok := msg.Attributes["Location"]
	if ok {
		logFileName += "_"
	}
	s, ok := msg.Attributes["Service"]
	if !ok {
		log.Println("Logger: Message missing Service")
		return
	}
	logFileName += s

	fullFileName := filepath.Join(logDirectory, logFileName)

	// now, append the line to the file
	// but first, global lock to ensure we don't mess up the file
	mu.Lock()
	defer mu.Unlock()

	// If this is message zero, and it is a repeat, discard it.
	append := true
	if filtering {
		msgNumS, ok := msg.Attributes["MsgNum"]
		if ok {
			msgNum, err := strconv.Atoi(msgNumS)
			if err == nil && msgNum == 0 {
				// we overwrite repeated copies of log message #0, but keep at least 1 a day.
				day := stampTime.Day()
				priorDay, _ := lastDay[logFileName]
				b, ok := repeatFilter[logFileName]
				if day == priorDay && ok && b {
					append = false
				}
				repeatFilter[logFileName] = true
				lastDay[logFileName] = day
			} else {
				repeatFilter[logFileName] = false
			}
		}
	}

	pos, ok := lastMessagePos[logFileName]
	if !ok {
		append = true
	}

	f, err := os.OpenFile(fullFileName, os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Printf("Logger: Cannot open for writing log file %s. err = %v", fullFileName, err)
		return
	}
	defer f.Close()

	size, err := f.Seek(0, 2)
	if err != nil {
		log.Printf("Logger: Cannot seek on log file %s. err = %v", fullFileName, err)
		return
	}

	if append || size <= pos {
		pos = size
	} else {
		_, err = f.Seek(pos, 0)
		if err != nil {
			log.Printf("Logger: Cannot second seek on log file %s. err = %v", fullFileName, err)
			return
		}
	}

	lastMessagePos[logFileName] = pos
	_, err = f.WriteString(formattedMsg)
	if err != nil {
		log.Printf("Logger: Error writing to file %s.  err = %v\n", fullFileName, err)
		return
	}
}
