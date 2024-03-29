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

	VCrucial LogVerbosity = iota
	VBasic
	VVerbose
	VExcessive
	//	additional

	VTemp  LogVerbosity = 0
	VStale LogVerbosity = 10

	// Topic
	//	raft basic
	TLeader   LogTopic = "LEAD"
	TAppend   LogTopic = "APND"
	TSnapshot LogTopic = "SNAP"

	TCandidate LogTopic = "CAND"
	TVote      LogTopic = "VOTE"

	TDemotion LogTopic = "DEMO"
	TTick     LogTopic = "TICK"
	TCommit   LogTopic = "CMIT"
	TApply    LogTopic = "APLY"

	TLogFail    LogTopic = "LOG0"
	TLogSuccess LogTopic = "LOG1"

	//	client
	TClient1 LogTopic = "CLT1"
	TClient2 LogTopic = "CLT2"

	//	servers
	TCtrler1   LogTopic = "CTR1"
	TCtrler2   LogTopic = "CTR2"
	TKVServer1 LogTopic = "KVS1"
	TKVServer2 LogTopic = "KVS2"

	TConfig1 LogTopic = "CNF1"
	TConfig2 LogTopic = "CNF2"

	//	tester
	TTester LogTopic = "TSTR"

	//	extra
	TError LogTopic = "ERRO"
	TWarn  LogTopic = "WARN"
	TTrace LogTopic = "TRCE"
)

type (
	LogVerbosity int
	LogTopic     string
)

// INIT CONFIGURATION

var logStart time.Time

var verbosity int

var raftVerbosity int
var kvVerbosity int
var ctrlrVerbosity int
var testVerbosity int

func getVerbosity(env string) int {
	v := os.Getenv(env)
	level := 0
	if v != "" {
		var err error
		level, err = strconv.Atoi(v)
		if err != nil {
			log.Fatalf("Invalid verbosity %v", v)
		}
	}
	if level != 0 {
		return level
	} else {
		return verbosity
	}
}

func init() {
	verbosity = getVerbosity("VERBOSE")

	raftVerbosity = getVerbosity("RV")
	kvVerbosity = getVerbosity("KV")
	ctrlrVerbosity = getVerbosity("CV")
	testVerbosity = getVerbosity("TV")

	logStart = time.Now()

	log.SetFlags(log.Flags() &^ (log.Ldate | log.Ltime))
}

// LOG FUNCTION

// raft

func LogRaft(verbosity LogVerbosity, topic LogTopic, peerId int, term int, format string, a ...interface{}) {
	if raftVerbosity >= int(verbosity) {
		_log("R", verbosity, topic, peerId, term, format, a...)
	}
}

// kv server

func LogKV(verbosity LogVerbosity, topic LogTopic, peerId int, format string, a ...interface{}) {
	if kvVerbosity >= int(verbosity) {
		_log("K", verbosity, topic, peerId, 0, format, a...)
	}
}

func LogKVClnt(verbosity LogVerbosity, topic LogTopic, peerId int, format string, a ...interface{}) {
	if kvVerbosity >= int(verbosity) {
		_log("C", verbosity, topic, peerId, 0, format, a...)
	}
}

// shard controller

func LogCtrler(verbosity LogVerbosity, topic LogTopic, peerId int, format string, a ...interface{}) {
	if ctrlrVerbosity >= int(verbosity) {
		_log("S", verbosity, topic, peerId, 0, format, a...)
	}
}

func LogCtrlerClnt(verbosity LogVerbosity, topic LogTopic, peerId int, format string, a ...interface{}) {
	if ctrlrVerbosity >= int(verbosity) {
		_log("C", verbosity, topic, peerId, 0, format, a...)
	}
}

// tester

func LogTest(verbosity LogVerbosity, topic LogTopic, peerId int, format string, a ...interface{}) {
	if testVerbosity >= int(verbosity) {
		_log("T", verbosity, topic, peerId, NA, format, a...)
	}
}

//
// a custom log function
//
func _log(role string, verbosity LogVerbosity, topic LogTopic, peerId int, term int, format string, a ...interface{}) {
	_time := time.Since(logStart).Microseconds()

	sec := _time / 1e6
	milliSec := (_time % 1e6) / 1e3
	microSec := (_time % 1e3) / 1e2

	timeFlag := fmt.Sprintf("%03d.%03d.%01d", sec, milliSec, microSec)

	prefix := fmt.Sprintf("%v %v %v%02d T%02d ", timeFlag, string(topic), role, peerId, term)
	format = prefix + format
	log.Printf(format, a...)
}
