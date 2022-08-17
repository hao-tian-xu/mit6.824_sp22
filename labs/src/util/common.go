package util

import "time"

const (
	NA = -1

	// timing
	MinInterval        = 10 * time.Millisecond
	HeartBeatsInterval = 50 * time.Millisecond

	// raft role
	Leader    = "Leader"
	Candidate = "Candidate"
	Follower  = "Follower"

	// server err
	OK             = "OK"
	ErrWrongLeader = "ErrWrongLeader"

	ErrNotApplied = "ErrNotApplied"
	ErrTimeout    = "ErrTimeout"
	ErrDuplicate  = "ErrDuplicate"
)

func Min(a int, b int) int {
	if a < b {
		return a
	}
	return b
}
