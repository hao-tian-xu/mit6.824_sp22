package raft

import "fmt"

const (
	rpcRequestVote   = "Raft.RequestVote"
	rpcAppendEntries = "Raft.AppendEntries"
)

// REQUEST VOTE RPC STRUCT

//
// example RequestVote RPC arguments structure.
// field names must start with capital letters!
//
type RequestVoteArgs struct {
	// Your data here (2A, 2B).
	Term         int // candidate’s term
	CandidateId  int // candidate requesting vote
	LastLogIndex int // index of candidate’s last log entry (§5.4)
	LastLogTerm  int //term of candidate’s last log entry (§5.4)
}

func (r *RequestVoteArgs) String() string {
	return fmt.Sprintf("lastLog ind%v/term%v in term %v", r.LastLogIndex, r.LastLogTerm, r.Term)
}

//
// example RequestVote RPC reply structure.
// field names must start with capital letters!
//
type RequestVoteReply struct {
	// Your data here (2A).
	Term        int
	VoteGranted bool
}

func (r *RequestVoteReply) String() string {
	return fmt.Sprintf("%v/term%v", r.VoteGranted, r.Term)
}

// APPEND ENTRIES RPC STRUCT

type AppendEntriesArgs struct {
	Term         int        // leader’s term
	LeaderId     int        // so follower can redirect clients
	PrevLogIndex int        // index of log entry immediately preceding new ones
	PrevLogTerm  int        // term of prevLogIndex entry
	Entries      []LogEntry // log entries to store (empty for heartbeat; may send more than one for efficiency)
	LeaderCommit int        // leader’s commitIndex
}

func (a AppendEntriesArgs) String() string {
	return fmt.Sprintf("prevLog ind%v/term%v in term %v, leaderCommit %v", a.PrevLogIndex, a.PrevLogTerm, a.Term, a.LeaderCommit)
}

type AppendEntriesReply struct {
	Term    int  // currentTerm, for leader to update itself
	Success bool // true if follower contained entry matching prevLogIndex and prevLogTerm
}

func (a AppendEntriesReply) String() string {
	return fmt.Sprintf("%v/term%v", a.Success, a.Term)
}
