package raft

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"time"
)

// INIT CONFIGURATION

var logStart time.Time
var logVerbosity int

func getVerbosity() int {
	v := os.Getenv("vRAFT")
	level := 0
	if v != "" {
		var err error
		level, err = strconv.Atoi(v)
		if err != nil {
			log.Fatalf("Invalid verbosity %v", v)
		}
	}
	return level
}

func init() {
	logVerbosity = getVerbosity()
	logStart = time.Now()

	log.SetFlags(log.Flags() &^ (log.Ldate | log.Ltime))
}

// LOG CONFIGURATION

type (
	LogVerbosity int
	LogTopic     string
)

//
// custom log function
//
func LogRaft(verbosity LogVerbosity, topic LogTopic, peerId int, format string, a ...interface{}) {
	if logVerbosity >= int(verbosity) {
		_time := time.Since(logStart).Microseconds()

		sec := _time / 1e6
		milliSec := (_time % 1e6) / 1e3
		microSec := (_time % 1e3) / 1e2

		timeFlag := fmt.Sprintf("%03d.%03d.%01d", sec, milliSec, microSec)

		prefix := fmt.Sprintf("%v %v S%02d ", timeFlag, string(topic), peerId)
		format = prefix + format
		log.Printf(format, a...)
	}
}
