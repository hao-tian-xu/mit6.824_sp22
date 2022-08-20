package util

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"time"
)

// VERBOSITY AND TOPIC

const (
	// Verbosity
	//	basic
	VBasic LogVerbosity = iota + 1
	VVerbose
	VExcessive
	//	additional
	VTemp  LogVerbosity = 0
	VStale LogVerbosity = 10

	// Topic
	//	raft basic
	TLeader     LogTopic = "LEAD"
	TCandidate  LogTopic = "CAND"
	TDemotion   LogTopic = "DEMO"
	TVote       LogTopic = "VOTE"
	TAppend     LogTopic = "APND"
	TTick       LogTopic = "TICK"
	TLogFail    LogTopic = "LOG0"
	TLogSuccess LogTopic = "LOG1"
	TCommit     LogTopic = "CMIT"
	TApply      LogTopic = "APLY"
	TSnapshot   LogTopic = "SNAP"
	//	extra
	TRedo  LogTopic = "REDO"
	TTrace LogTopic = "TRCE"
	TError LogTopic = "ERRO"
	TWarn  LogTopic = "WARN"
	//	shard controller
	TClient1 LogTopic = "CLT1"
	TClient2 LogTopic = "CLT2"
	TCtrler1 LogTopic = "CTR1"
	TCtrler2 LogTopic = "CTR2"
	//	kv
	TKVServer1 LogTopic = "KVS1"
	TKVServer2 LogTopic = "KVS2"
	//	tester
	TTester LogTopic = "TSTR"
)

type (
	LogVerbosity int
	LogTopic     string
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

// LOG FUNCTION

func LogTest(verbosity LogVerbosity, topic LogTopic, peerId int, format string, a ...interface{}) {
	_log("T", verbosity, topic, peerId, NA, format, a...)
}

//
// raft log wrapper
//
func LogRaft(verbosity LogVerbosity, topic LogTopic, peerId int, term int, format string, a ...interface{}) {
	_log("R", 10, topic, peerId, term, format, a...)
}

//
// controller log wrapper
//
func LogCtrler(verbosity LogVerbosity, topic LogTopic, peerId int, format string, a ...interface{}) {
	_log("S", verbosity, topic, peerId, 0, format, a...)
}

//
// controller client log wrapper
//
func LogCtrlerClnt(verbosity LogVerbosity, topic LogTopic, peerId int, format string, a ...interface{}) {
	_log("C", verbosity, topic, peerId, 0, format, a...)
}

//
// a custom log function
//
func _log(role string, verbosity LogVerbosity, topic LogTopic, peerId int, term int, format string, a ...interface{}) {
	if logVerbosity >= int(verbosity) {
		_time := time.Since(logStart).Microseconds()

		sec := _time / 1e6
		milliSec := (_time % 1e6) / 1e3
		microSec := (_time % 1e3) / 1e2

		timeFlag := fmt.Sprintf("%03d.%03d.%01d", sec, milliSec, microSec)

		prefix := fmt.Sprintf("%v %v %v%02d T%02d ", timeFlag, string(topic), role, peerId, term)
		format = prefix + format
		log.Printf(format, a...)
	}
}
