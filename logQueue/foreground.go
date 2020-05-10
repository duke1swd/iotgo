/*
 * System for enqueing log messages and then later, when possible
 * shipping them off somewhere
 */

package logQueue

import (
	"io/ioutil"
	"os"
	"fmt"
	"context"
	"errors"
)

const workingDir = "/tmp/logQueue"
var (
	cancelFunc context.CancelFunc
)

type LogSender func(s string, c context.Context) bool

/*
 * Initializes the system.  Spawns a thread
 * that does the work
 */
func Start(sender LogSender) error {
	var cancelcontext context.Context

	// Already running?
	if cancelFunc != nil {
		return errors.New("logQueue already started")
	}

	// does the working directory exist?
	_, err := ioutil.ReadDir(workingDir)
	if err != nil {
		// No.  Try making it
		err = os.Mkdir(workingDir, 0755)
		if err != nil {
			// Nope.  We are done.
			return fmt.Errorf("Trying to mkdir %s got error %w",
				workingDir,
				err)
		}
	}

	// spawn the thread that will pump the enqueued messages
	ctx := context.Background()
	cancelcontext, cancelFunc = context.WithCancel(ctx)
	go backgroundLogThread(cancelcontext, sender)
	return nil
}

/*
 * Stops the background thread for a clean shutdown
 */
func Stop() {
	if cancelFunc != nil {
		cancelFunc()
	}

	cancelFunc = nil
}

func backgroundLogThread(c context.Context, sender LogSender) {
	// Not yet implemented
}
