package util

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"time"
)

// CONST

const (
	// log verbosity level
	Basic LogVerbosity = iota + 1
	Verbose
	Excessive
	//  additional
	Temp  LogVerbosity = 0
	Stale LogVerbosity = 10

	// log topic
	Apply LogTopic = "APLY"

	Redo  LogTopic = "REDO"
	Trace LogTopic = "TRCE"
	Error LogTopic = "ERRO"
	Warn  LogTopic = "WARN"

	Client LogTopic = "CLNT"
	Ctrler LogTopic = "CTLR"
)

// INIT CONFIGURATION

var logStart time.Time
var logVerbosity int

func getVerbosity() int {
	v := os.Getenv("VS")
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
// controller warapper
//
func LogCtrlerClnt(verbosity LogVerbosity, topic LogTopic, peerId int, format string, a ...interface{}) {
	_log("C", verbosity, topic, peerId, format, a...)
}

func LogCtrler(verbosity LogVerbosity, topic LogTopic, peerId int, format string, a ...interface{}) {
	_log("S", verbosity, topic, peerId, format, a...)
}

//
// custom log function
//
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
