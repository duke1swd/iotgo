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
	"time"
	"strconv"
	"strings"
)

const workingDir = "/tmp/logQueue"

type LogSender func(t, s string, c context.Context) bool

var (
	running bool
	seqn int64
	epoch time.Time
	debugMode bool
)

/*
 * Initializes the system.  Spawns a thread
 * that does the work
 */
func Start(myContext context.Context, sender LogSender) error {
	// Already running?
	if running {
		return errors.New("logQueue already started")
	}

	// does the working directory exist?
	_, err := ioutil.ReadDir(workingDir)
	if err != nil {
		// No.  Try making it
		err = os.Mkdir(workingDir, 0755)
		if err != nil {
			// Nope.  We are done.
			return fmt.Errorf("Trying to mkdir %s got error %v",
				workingDir,
				err)
		}
	}
	clean(false)

	epoch, err = time.Parse("2006-Jan-02 MST", "2018-Nov-01 EDT")
	if err != nil {
		return fmt.Errorf("failed to get epoch")
	}


	// spawn the thread that will pump the enqueued messages
	go backgroundLogThread(myContext, sender)
	running = true
	return nil
}

func Log(s string) error {
	if ! running {
		return errors.New("logQueue not running")
	}

	// write the log message to a temporary file
	tempFileName := workingDir + "/_" + strconv.FormatInt(seqn, 10)
	tempFile, err := os.OpenFile(tempFileName, os.O_EXCL | os.O_CREATE | os.O_WRONLY, 0600)
	if err != nil {
		return fmt.Errorf("Cannot creat temp file %s: %v", tempFileName, err)
	}
	_, err = tempFile.WriteString(s)
	if err != nil {
		return fmt.Errorf("Cannot write temp file %s: %v", tempFileName, err)
	}
	tempFile.Close()

	// rename the temp file to its final name
	now := int64(time.Since(epoch) / time.Second)
	logFileName := workingDir + "/" + fmt.Sprintf("%d_%d", now, seqn)
	seqn += 1
	err = os.Rename(tempFileName, logFileName)
	if err != nil {
		return fmt.Errorf("Cannot rename temp file %s to %s: %v", tempFileName, logFileName, err)
	}

	return nil
}

/*
 * Clean junk out of the log directory.
 * If the arg is true, cleans everything out.
 */

func clean(very bool) {
	files, err := ioutil.ReadDir(workingDir)
	if err != nil {
		return
	}

	for _, f := range(files) {
		shortName := f.Name()
		file := workingDir + "/" + shortName

		// ignore files whose name begins with "_"
		if very || strings.HasPrefix(shortName, "_") {
			os.Remove(file)
		}
	}
}
