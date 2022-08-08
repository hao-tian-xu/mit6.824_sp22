package util

import "time"

const (
	NA = -1

	// timing
	MinInterval        = 10 * time.Millisecond
	HeartBeatsInterval = 100 * time.Millisecond

	// server err
	OK             = "OK"
	ErrWrongLeader = "ErrWrongLeader"

	ErrNotApplied = "ErrNotApplied"
	ErrTimeout    = "ErrTimeout"
	ErrDuplicate  = "ErrDuplicate"
)
