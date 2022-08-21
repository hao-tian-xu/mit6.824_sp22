package util

import "time"

const (
	NA = -1

	// timing
	HeartbeatsInterval = 50 * time.Millisecond
	MinTimeout         = 8  // as number of heartbeats interval
	MaxTimeout         = 16 // as number of heartbeats interval

	MinInterval = 10 * time.Millisecond

	// raft role
	Leader    = "Leader"
	Candidate = "Candidate"
	Follower  = "Follower"

	// server err
	OK             = "OK"
	ErrWrongLeader = "ErrWrongLeader"
	ErrNoKey       = "ErrNoKey"

	ErrNotApplied = "ErrNotApplied"
	ErrTimeout    = "ErrTimeout"
	ErrDuplicate  = "ErrDuplicate"
)

type Err string

func Min(a int, b int) int {
	if a < b {
		return a
	}
	return b
}

func Max(a int, b int) int {
	if a > b {
		return a
	}
	return b
}
