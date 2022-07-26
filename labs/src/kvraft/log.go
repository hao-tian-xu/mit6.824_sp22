package kvraft

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
	v := os.Getenv("vKV")
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

var topicClient = map[string]bool{"CLNT": true}

//
// custom log function
//
func LogClient(verbosity LogVerbosity, topic LogTopic, peerId int, format string, a ...interface{}) {
	_log("C", verbosity, topic, peerId, format, a...)
}

func LogKV(verbosity LogVerbosity, topic LogTopic, peerId int, format string, a ...interface{}) {
	_log("K", verbosity, topic, peerId, format, a...)
}

func _log(role string, verbosity LogVerbosity, topic LogTopic, peerId int, format string, a ...interface{}) {
	if logVerbosity >= int(verbosity) {
		_time := time.Since(logStart).Microseconds()

		sec := _time / 1e6
		milliSec := (_time % 1e6) / 1e3
		microSec := (_time % 1e3) / 1e2

		timeFlag := fmt.Sprintf("%03d.%03d.%01d", sec, milliSec, microSec)

		prefix := fmt.Sprintf("%v %v %v%02d ", timeFlag, string(topic), role, peerId)
		format = prefix + format
		log.Printf(format, a...)
	}
}
