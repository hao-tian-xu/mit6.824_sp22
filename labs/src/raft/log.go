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
	v := os.Getenv("VERBOSE")
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
		time := time.Since(logStart).Microseconds()
		time /= 100

		prefix := fmt.Sprintf("%06d %v S%02d ", time, string(topic), peerId)
		format = prefix + format
		log.Printf(format, a...)
	}
}
