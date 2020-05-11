package logQueue

import (
	"io/ioutil"
	"context"
	"time"
	"os"
	"strings"
)

const timeToSend = 10	// timeout in seconds
const timeToWait = 600	// time between directory scans, in seconds

/*
 * This function is the background thread.
 *
 * It wakes up every minute
 * When it wakes up it scans the directory.
 * For each file in the directory, it reads the file into a string,
 * and calls the sender function with the file name and file contents.
 *  (Note: the file name is presumed to be a timestamp)
 * If the sender returns false, it goes back to sleep.
 * If the sender returns true, it removes the corresponding file
 * and goes on to the next file.
 *
 * The context passed to the sender has a timeout.  If that context
 * is cancelled or times out, sender is to return false.
 *
 * The thread exits when the context is cancelled
 */

func backgroundLogThread(c context.Context, sender LogSender) {
	running = true
	defer func() {
		running = false
	}()

	for {
		files, err := ioutil.ReadDir(workingDir)
		if err != nil {
			return
		}
		for _, f := range(files) {
			file := f.Name()
			// ignore files whose name begins with "_"
			if strings.HasPrefix(file, "_") {
				continue
			}

			content, err := ioutil.ReadFile(file)
			if err != nil {
				// for some reason could not read the file.
				// try to remove it and then move on
				err := os.Remove(file)
				if err != nil {
					// if there is an error, abort. Prevent infinite loop this way
					return
				}
				continue
			}
			text := string(content)
			ctx, cf := context.WithTimeout(c, timeToSend * time.Second)

			// send the log message off into the world
			r := sender(file, text, ctx)
			cf()
			if !r {
				// sender failed
				break;
			}
			// worked.  delete the message and loop
			err = os.Remove(file)
			if err != nil {
				// if there is an error, abort. Prevent infinite loop this way
				return
			}
		}

		// Either we processed all the files or the sender failed
		// Wait until 5 minutes or until the incoming context is cancelled.
		t := timeToWait
		if (debugMode) {
			t = 10
		}
		ctx, cf := context.WithTimeout(c, time.Duration(t) * time.Second)
		<- ctx.Done()
		if c.Err() == context.Canceled {
			// If the incoming context has been cancelled, we are instructed to stop.
			cf()
			return;
		}
		cf()
	}
}
