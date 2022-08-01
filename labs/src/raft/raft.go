package raft

//
// this is an outline of the API that raft must expose to
// the service (or tester). see comments below for
// each of these functions for more details.
//
// rf = Make(...)
//   create a new Raft server.
// rf.Start(command interface{}) (index, term, isleader)
//   start agreement on a new log entry
// rf.GetState() (term, isLeader)
//   ask a Raft for its current term, and whether it thinks it is leader
// ApplyMsg
//   each time a new entry is committed to the log, each Raft peer
//   should send an ApplyMsg to the service (or tester)
//   in the same server.
//

import (
	"6.824/labgob"
	"bytes"
	"log"
	"math/rand"
	"sort"
	"time"

	//	"bytes"
	"sync"
	"sync/atomic"

	//	"6.824/labgob"
	"6.824/labrpc"
)

// NA AND LOG CONSTANT

const (
	_NA = -1

	// log verbosity level
	vBasic LogVerbosity = iota
	vVerbose
	vExcessive

	// log topic
	tError      LogTopic = "ERRO"
	tLeader     LogTopic = "LEAD"
	tCandidate  LogTopic = "CAND"
	tDemotion   LogTopic = "DEMO"
	tTerm       LogTopic = "TERM"
	tLogFail    LogTopic = "LOG0"
	tLogSuccess LogTopic = "LOG1"
	tCommit     LogTopic = "CMIT"
	tSnapshot   LogTopic = "SNAP"
	tTrace      LogTopic = "TRCE"
)

// APPLYMSG STRUCTURE

//
// as each Raft peer becomes aware that successive log entries are
// committed, the peer should send an ApplyMsg to the service (or
// tester) on the same server, via the applyCh passed to Make(). set
// CommandValid to true to indicate that the ApplyMsg contains a newly
// committed log entry.
//
// in part 2D you'll want to send other kinds of messages (e.g.,
// snapshots) on the applyCh, but set CommandValid to false for these
// other uses.
//
type ApplyMsg struct {
	CommandValid bool
	Command      interface{}
	CommandIndex int

	// For 2D:
	SnapshotValid bool
	Snapshot      []byte
	SnapshotTerm  int
	SnapshotIndex int
}

// RAFT STRUCTURE

//
// A Go object implementing a single Raft peer.
//
type Raft struct {
	sync.Mutex                     // Lock to protect shared access to this peer's state
	peers      []*labrpc.ClientEnd // RPC end points of all peers
	persister  *Persister          // Object to hold this peer's persisted state
	me         int                 // this peer's index into peers[]
	dead       int32               // set by Kill()

	// Your data here (2A, 2B, 2C).
	// Look at the paper's Figure 2 for a description of what
	// state a Raft server must maintain.

	// Persistent state on all servers: (Updated on stable storage before responding to RPCs)
	currentTerm int
	votedFor    int
	log         []*LogEntry
	// Volatile state on all servers:
	commitIndex int
	lastApplied int
	// Volatile state on leaders: (Reinitialized after election)
	nextIndex  []int
	matchIndex []int

	// Additional persistent state
	firstLogIndex int
	snapshot      []byte
	// Additional volatile state
	nPeers       int
	timeoutStart time.Time
	role         peerRole
	termLeader   map[int]int
	// ApplyMsg
	applyCh      chan ApplyMsg
	condLock     sync.Mutex
	cond         *sync.Cond
	applyMsgList []*ApplyMsg
}

type LogEntry struct {
	Term    int
	Command interface{}
}

type peerRole int

const (
	// peer role
	rLeader peerRole = iota
	rCandidate
	rFollower
)

// TIMING

const (
	// general loop
	_LoopInterval = 10 * time.Millisecond

	// heartbeats
	_HeartbeatsInterval = 100 * time.Millisecond

	// timeout
	_TimeoutMin = 1000
	_TimeoutMax = 1500
)

func randomTimeout() time.Duration {
	return time.Millisecond * time.Duration(_TimeoutMin+rand.Intn(_TimeoutMax-_TimeoutMin))
}

// HELPER

//
// Custom rf.Lock(), get the lock only if rf.currentTerm keeps the same and return is true
//
func (rf *Raft) termLock(term int) bool {
	rf.Lock()
	if rf.currentTerm != term {
		rf.Unlock()
		return false
	}
	return true
}

//
// All Servers: If RPC request or response contains term T > currentTerm: set currentTerm = T, convert to follower
// must be called with rf.Lock()
//
func (rf *Raft) demotion(term int) {
	rf.currentTerm = term
	rf.votedFor = _NA
	rf.persist()
	rf.role = rFollower
}

//
// Return lastLogIndex, lastLogTerm.
// must be called with rf.Lock()
//
func (rf *Raft) lastLogStat() (int, int) {
	lastLogSliceIndex := len(rf.log) - 1
	lastLogIndex := rf.firstLogIndex + lastLogSliceIndex
	lastLogTerm := rf.log[lastLogSliceIndex].Term
	return lastLogIndex, lastLogTerm
}

//
// Calculate index in rf.log with logIndex
// must be called with rf.Lock()
//
func (rf *Raft) logSliceIndex(logIndex int) int {
	return logIndex - rf.firstLogIndex
}

//
// Get LogEntry with logIndex
// must be called with rf.Lock()
//
func (rf *Raft) getLog(logIndex int) *LogEntry {
	return rf.log[rf.logSliceIndex(logIndex)]
}

func min(a int, b int) int {
	if a < b {
		return a
	}
	return b
}

// GET STATE

// return currentTerm and whether this server
// believes it is the leader.
func (rf *Raft) GetState() (int, bool) {

	// Your code here (2A).
	rf.Lock()
	defer rf.Unlock()
	return rf.currentTerm, rf.role == rLeader
}

// PERSIST

//
// save Raft's persistent state to stable storage,
// where it can later be retrieved after a crash and restart.
// see paper's Figure 2 for a description of what should be persistent.
//
func (rf *Raft) persist() {
	// Your code here (2C).
	w := new(bytes.Buffer)
	e := labgob.NewEncoder(w)
	if e.Encode(rf.currentTerm) != nil ||
		e.Encode(rf.votedFor) != nil ||
		e.Encode(rf.log) != nil ||
		e.Encode(rf.firstLogIndex) != nil {

		log.Fatalln("persist() failed!")
	} else {
		data := w.Bytes()
		rf.persister.SaveStateAndSnapshot(data, rf.snapshot)
	}
}

//
// restore previously persisted state.
//
func (rf *Raft) readPersist(data []byte) {
	if data == nil || len(data) < 1 { // bootstrap without any state?
		return
	}
	// Your code here (2C).
	r := bytes.NewBuffer(data)
	d := labgob.NewDecoder(r)
	var currentTerm int
	var votedFor int
	var _log []*LogEntry
	var firstLogIndex int
	if d.Decode(&currentTerm) != nil ||
		d.Decode(&votedFor) != nil ||
		d.Decode(&_log) != nil ||
		d.Decode(&firstLogIndex) != nil {

		log.Fatalln("readPersist() failed!")
	} else {
		rf.currentTerm = currentTerm
		rf.votedFor = votedFor
		rf.log = _log
		rf.firstLogIndex = firstLogIndex
		rf.snapshot = rf.persister.ReadSnapshot()

		rf.commitIndex = rf.firstLogIndex
	}
}

// SNAPSHOT

//
// A service wants to switch to snapshot.  Only do so if Raft hasn't
// have more recent info since it communicate the snapshot on applyCh.
//
func (rf *Raft) CondInstallSnapshot(lastIncludedTerm int, lastIncludedIndex int, snapshot []byte) bool {

	// Your code here (2D).

	return true
}

// the service says it has created a snapshot that has
// all info up to and including index. this means the
// service no longer needs the log through (and including)
// that index. Raft should now trim its log as much as possible.
func (rf *Raft) Snapshot(index int, snapshot []byte) {
	// Your code here (2D).
	rf.Lock()
	rf.trimLog(index)
	rf.snapshot = snapshot
	rf.persist()
	rf.Unlock()

	LogRaft(vBasic, tSnapshot, rf.me, "snapshot at %v", index)
}

//
// Trim rf.log through index
// must be called with rf.Lock()
//
func (rf *Raft) trimLog(index int) {
	// 2D Hint: Raft must discard old log entries
	for i := 0; i < index-rf.firstLogIndex; i++ {
		rf.log[i] = nil
	}

	rf.log = rf.log[index-rf.firstLogIndex:]
	rf.firstLogIndex = index
}

// REQUEST VOTE RPC

//
// RequestVote RPC arguments structure.
//
type RequestVoteArgs struct {
	// Your data here (2A, 2B).
	Term         int
	CandidateId  int
	LastLogIndex int
	LastLogTerm  int
}

//
// RequestVote RPC reply structure.
//
type RequestVoteReply struct {
	// Your data here (2A).
	Term        int
	VoteGranted bool
}

//
// RequestVote RPC handler.
//
func (rf *Raft) RequestVote(args *RequestVoteArgs, reply *RequestVoteReply) {
	// Your code here (2A, 2B).
	rf.Lock()
	defer rf.Unlock()

	// All Servers: If RPC request or response contains term T > currentTerm: set currentTerm = T, convert to follower
	if args.Term > rf.currentTerm {
		rf.demotion(args.Term)
		go LogRaft(vBasic, tTerm, rf.me, "F in T%v (RequestVote)/C%v\n", rf.currentTerm, args.CandidateId)
	}

	reply.Term = rf.currentTerm

	lastLogIndex, lastLogTerm := rf.lastLogStat()

	// Receiver implementation: Reply false if term < currentTerm
	if args.Term >= rf.currentTerm &&
		// Receiver implementation: If votedFor is null or candidateId,
		(rf.votedFor == _NA || rf.votedFor == args.CandidateId) &&
		//		and candidate’s log is at least as up-to-date as receiver’s log, grant vote
		//		5.4.1 Election restriction: If the logs have last entries with different terms,
		//			then the log with the later term is more up-to-date.
		//			If the logs end with the same term, then whichever log is longer is more up-to-date.
		(args.LastLogTerm > lastLogTerm ||
			(args.LastLogTerm == lastLogTerm && args.LastLogIndex >= lastLogIndex)) {

		rf.votedFor = args.CandidateId
		rf.persist()
		reply.VoteGranted = true

		// Followers: reset election timeout if granting vote to candidate
		rf.timeoutStart = time.Now()
	} else {
		reply.VoteGranted = false
	}
}

//
// send a RequestVote RPC to a server and process the reply
//
func (rf *Raft) sendRequestVote(server int, term int, lastLogIndex int, lastLogTerm int, voteCounter *voteCounter) {
	args := &RequestVoteArgs{term, rf.me, lastLogIndex, lastLogTerm}
	reply := &RequestVoteReply{}

	ok := rf.peers[server].Call("Raft.RequestVote", args, reply)

	if !rf.termLock(term) {
		return
	}

	switch true {
	case !ok:
		defer LogRaft(vVerbose, tError, rf.me, "C send RequestVote to V#%v failed!\n", server)
	case reply.Term > rf.currentTerm:
		// All Servers: If RPC request or response contains term T > currentTerm:
		//		set currentTerm = T, convert to follower
		rf.demotion(reply.Term)
		defer LogRaft(vBasic, tTerm, rf.me, "C->F in T%v/S%v (sendRequestVote)\n", reply.Term, server)
	case reply.VoteGranted == true:
		defer func() {
			voteCounter.Lock()
			voteCounter.n += 1
			voteCounter.Unlock()
			LogRaft(vBasic, tCandidate, rf.me, "C votedBy V%v! (sendRequestVote)\n", server)
		}()
	default:
		defer LogRaft(vBasic, tCandidate, rf.me, "C notVotedBy V%v... (sendRequestVote)\n", server)
	}
	rf.Unlock()
}

// APPEND ENTRIES RPC

//
// AppendEntries RPC arguments structure
//
type AppendEntriesArgs struct {
	Term         int
	LeaderId     int
	PrevLogIndex int
	PrevLogTerm  int
	Entries      []*LogEntry
	LeaderCommit int
}

//
// AppendEntries RPC reply structure
//
type AppendEntriesReply struct {
	Term    int
	Success bool

	// Back Up Quickly
	Inconsistency Inconsistency
}

type Inconsistency struct {
	// Mismatch
	Term           int
	TermFirstIndex int
	// No entry
	LastLogIndex int
	NoEntry      bool
}

//
// AppendEntries RPC handler
//
func (rf *Raft) AppendEntries(args *AppendEntriesArgs, reply *AppendEntriesReply) {
	rf.Lock()
	defer rf.Unlock()

	reply.Term = rf.currentTerm

	// Candidates: If AppendEntries RPC received from new leader: convert to follower
	if rf.role == rCandidate && rf.currentTerm <= args.Term {
		rf.role = rFollower
		rf.termLeader[rf.currentTerm] = args.LeaderId

		go LogRaft(vBasic, tDemotion, rf.me, "C -> F.. (AppendEntries)/L%v\n", args.LeaderId)
	}

	// All Servers: If RPC request or response contains term T > currentTerm: set currentTerm = T, convert to follower
	if args.Term > rf.currentTerm {
		rf.demotion(args.Term)

		go LogRaft(vBasic, tTerm, rf.me, "F in T%v (AppendEntries)/L%v\n", rf.currentTerm, args.LeaderId)
	}

	// Receiver implementation 1: Reply false if term < currentTerm
	if args.Term < rf.currentTerm {
		reply.Success = false
		return
	}

	// Followers: reset election timeout if receiving AppendEntries RPC from current leader
	rf.timeoutStart = time.Now()
	if _, ok := rf.termLeader[rf.currentTerm]; !ok {
		rf.termLeader[rf.currentTerm] = args.LeaderId
	}

	// Receiver implementation 2: Reply false if log doesn’t contain an entry
	//		at prevLogIndex whose term matches prevLogTerm
	lastLogIndex, _ := rf.lastLogStat()
	if lastLogIndex < args.PrevLogIndex {
		reply.Success = false
		reply.Inconsistency = Inconsistency{LastLogIndex: lastLogIndex, NoEntry: true}
		return
	}
	prevLogTerm := rf.getLog(args.PrevLogIndex).Term
	if prevLogTerm != args.PrevLogTerm {
		reply.Success = false
		termFirstIndex := args.PrevLogIndex
		for {
			termFirstIndex--
			if rf.getLog(termFirstIndex).Term != prevLogTerm ||
				// Snapshot (Additional): traced back to the 0 index of log slice
				termFirstIndex == rf.firstLogIndex {
				termFirstIndex++
				break
			}
		}
		reply.Inconsistency = Inconsistency{Term: prevLogTerm, TermFirstIndex: termFirstIndex}
		return
	}
	// Receiver implementation 4: Append any new entries not already in the log
	reply.Success = true
	if len(args.Entries) > 0 {
		for i, entry := range args.Entries {
			index := args.PrevLogIndex + 1 + i
			if lastLogIndex >= index {
				// Receiver implementation 3: If an existing entry conflicts with a new one
				//		(same index but different terms), delete the existing entry and all that follow it
				if entry.Term != rf.getLog(index).Term {
					rf.log = append(rf.log[:rf.logSliceIndex(index)], args.Entries[i:]...)
					rf.persist()
					break
				}
			} else {
				rf.log = append(rf.log, args.Entries[i:]...)
				rf.persist()
				break
			}
		}
		lastLogIndex, _ = rf.lastLogStat()
		go LogRaft(vBasic, tLogSuccess, rf.me, "log%v <- L%v! (AppendEntries)", lastLogIndex, args.LeaderId)
	}
	// Receiver implementation 5: If leaderCommit > commitIndex,
	//		set commitIndex = min(leaderCommit, index of last new entry)
	if args.LeaderCommit > rf.commitIndex {
		LogRaft(vExcessive, tTrace, rf.me, "1stLog-%v, commit-%v, leaderCommit-%v (AppendEntries)", rf.firstLogIndex, rf.commitIndex, args.LeaderCommit)
		rf.sendApplyMsg(min(args.LeaderCommit, lastLogIndex), false)

		go LogRaft(vBasic, tCommit, rf.me, "commit%v <- L%v! (AppendEntries)", rf.commitIndex, args.LeaderId)
	}
}

//
// Send AppendEntries RPCs to all other servers
//
func (rf *Raft) sendAppendEntries(term int, initial bool) {
	for i := 0; i < rf.nPeers; i++ {
		if i != rf.me {
			go rf._sendAppendEntries(i, term, initial)
		}
	}
}

//
// Send an AppendEntries RPC to a server and process the reply
//
func (rf *Raft) _sendAppendEntries(server int, term int, initial bool) {
	if !rf.termLock(term) {
		return
	}
	lastLogIndex, _ := rf.lastLogStat()
	nextIndex := rf.nextIndex[server]
	if nextIndex <= rf.firstLogIndex {
		nextIndex = rf.firstLogIndex + 1
	}
	prevLogIndex := nextIndex - 1
	prevLogTerm := rf.getLog(prevLogIndex).Term
	leaderCommit := rf.commitIndex
	// Leaders: If last log index ≥ nextIndex for a follower:
	//		send AppendEntries RPC with log entries starting at nextIndex
	entries := make([]*LogEntry, 0)
	if !initial && lastLogIndex >= nextIndex {
		entries = append(entries, rf.log[rf.logSliceIndex(nextIndex):]...)
	}
	rf.Unlock()

	args := &AppendEntriesArgs{term, rf.me, prevLogIndex, prevLogTerm, entries, leaderCommit}
	reply := &AppendEntriesReply{}

	ok := rf.peers[server].Call("Raft.AppendEntries", args, reply)

	if !rf.termLock(term) {
		return
	}

	switch true {
	case !ok:
		defer LogRaft(vVerbose, tError, rf.me, "L/T%v send AppendEntries to F%v failed!\n", term, server)
	case reply.Success:
		// Leaders: If successful: update nextIndex and matchIndex for follower
		rf.nextIndex[server] = lastLogIndex + 1
		rf.matchIndex[server] = lastLogIndex
	case reply.Term > rf.currentTerm:
		// All Servers: If RPC request or response contains term T > currentTerm: set currentTerm = T, convert to follower
		rf.demotion(reply.Term)
		defer LogRaft(vBasic, tTerm, rf.me, "L->F in T%v/S%v (_sendAppendEntries)\n", reply.Term, server)
	case reply.Inconsistency.NoEntry:
		// Leaders: If AppendEntries fails because of log inconsistency: decrement nextIndex and retry
		rf.nextIndex[server] = reply.Inconsistency.LastLogIndex + 1
		defer LogRaft(vBasic, tLogFail, rf.me, "L: log%v empty/F%v (_sendAppendEntries)\n", prevLogIndex, server)
	case reply.Inconsistency.Term != 0:
		// Leaders: If AppendEntries fails because of log inconsistency: decrement nextIndex and retry
		if reply.Inconsistency.Term > prevLogTerm {
			// 1. follower prevLog w/ larger term, to remove all
			rf.nextIndex[server] = reply.Inconsistency.TermFirstIndex
			defer LogRaft(vBasic, tLogFail, rf.me, "L: log%v mismatch-L/F%v (_sendAppendEntries)\n", prevLogIndex, server)
		} else {
			// 2. follower prevLog w/ smaller term, to remove neccessary
			firstIndex := reply.Inconsistency.TermFirstIndex
			if firstIndex <= rf.firstLogIndex {
				firstIndex = rf.firstLogIndex
			}
			for {
				if rf.getLog(firstIndex).Term != reply.Inconsistency.Term {
					break
				}
				firstIndex++
			}
			rf.nextIndex[server] = firstIndex
			defer LogRaft(vBasic, tLogFail, rf.me, "L: log%v mismatch-S/F%v (_sendAppendEntries)\n", prevLogIndex, server)
		}
	default:
		log.Fatalln("log failed with unkown reason!")
	}

	if rf.nextIndex[server] <= rf.firstLogIndex {
		go rf.sendInstallSnapShot(server, rf.currentTerm)
		defer LogRaft(vBasic, tSnapshot, rf.me, "L: snapShot->F%v\n", server)
	}

	rf.Unlock()
}

// INSTALL SNAPSHOT RPC

type InstallSnapshotArgs struct {
	Term              int
	LeaderId          int
	LastIncludedIndex int
	LastIncludedTerm  int
	Data              []byte

	// Additional
	LastIncludedLogEntry *LogEntry
}

type InstallSnapshotReply struct {
	Term int

	// Additional
	ReceiverId int
}

func (rf *Raft) InstallSnapshot(args *InstallSnapshotArgs, reply *InstallSnapshotReply) {
	rf.Lock()
	defer rf.Unlock()

	reply.Term = rf.currentTerm
	reply.ReceiverId = rf.me

	// All Servers: If RPC request or response contains term T > currentTerm: set currentTerm = T, convert to follower
	if args.Term > rf.currentTerm {
		rf.demotion(args.Term)

		go LogRaft(vBasic, tTerm, rf.me, "F in T%v (InstallSnapshot)/L%v\n", rf.currentTerm, args.LeaderId)
	}

	// Receiver implementation 1: Reply immediately if term < currentTerm
	if args.Term < rf.currentTerm {
		return
	}

	// Followers Additional: reset election timeout if receiving InstallSnapshot RPC from current leader
	rf.timeoutStart = time.Now()

	// Receiver Implementation 5: Save snapshot file
	rf.snapshot = args.Data
	// Receiver Implementation 6: If existing log entry has same index and term as snapshot’s last included entry,
	//		retain log entries following it and reply
	lastLogIndex, _ := rf.lastLogStat()
	if lastLogIndex >= args.LastIncludedIndex && rf.getLog(args.LastIncludedIndex).Term == args.LastIncludedTerm {
		rf.trimLog(args.LastIncludedIndex)
		rf.persist()
		return
	}
	// Receiver Implementation 7: Discard the entire log
	rf.log = []*LogEntry{args.LastIncludedLogEntry}
	rf.firstLogIndex = args.LastIncludedIndex

	rf.persist()

	// Receiver Implementation 8: Reset state machine using snapshot contents (and load snapshot’s cluster configuration)
	if args.LastIncludedIndex > rf.commitIndex {
		rf.sendApplyMsg(args.LastIncludedIndex, true)
		rf.commitIndex = args.LastIncludedIndex
	}
}

func (rf *Raft) sendInstallSnapShot(server int, term int) {
	if !rf.termLock(term) {
		return
	}
	lastIncludedIndex := rf.firstLogIndex
	lastIncludedTerm := rf.log[0].Term
	data := clone(rf.snapshot)
	lastIncludedLogEntry := rf.log[0]
	rf.Unlock()

	args := &InstallSnapshotArgs{term, rf.me, lastIncludedIndex, lastIncludedTerm, data, lastIncludedLogEntry}
	reply := &InstallSnapshotReply{}

	ok := rf.peers[server].Call("Raft.InstallSnapshot", args, reply)

	if !rf.termLock(term) {
		return
	}

	switch true {
	case !ok:
		defer LogRaft(vVerbose, tError, rf.me, "L/T%v send InstallSnapshot to F%v failed!\n", term, server)
	case reply.Term > rf.currentTerm:
		rf.demotion(reply.Term)
		defer LogRaft(vBasic, tTerm, rf.me, "L->F in T%v/S%v (sendInstallSnapShot)\n", reply.Term, reply.ReceiverId)
	}

	rf.Unlock()

}

// START

//
// the service using Raft (e.g. a k/v server) wants to start
// agreement on the next command to be appended to Raft's log. if this
// server isn't the leader, returns false. otherwise start the
// agreement and return immediately. there is no guarantee that this
// command will ever be committed to the Raft log, since the leader
// may fail or lose an election. even if the Raft instance has been killed,
// this function should return gracefully.
//
// the first return value is the index that the command will appear at
// if it's ever committed. the second return value is the current
// term. the third return value is true if this server believes it is
// the leader.
//
func (rf *Raft) Start(command interface{}) (int, int, bool) {

	// Your code here (2B).
	rf.Lock()
	defer rf.Unlock()
	if rf.role == rLeader {
		// Leader: If command received from client: append entry to local log
		defer rf.appendCommand(command)
		index, _ := rf.lastLogStat()
		index++
		return index, rf.currentTerm, true
	} else {
		return _NA, _NA, false
	}
}

//
// must be called with rf lock
//
func (rf *Raft) appendCommand(command interface{}) {
	logEntry := LogEntry{rf.currentTerm, command}
	rf.log = append(rf.log, &logEntry)
	rf.persist()

	lastLogIndex, _ := rf.lastLogStat()
	go LogRaft(vBasic, tLeader, rf.me, "log%v (appendCommand)\n", lastLogIndex)
}

//
// Wrapper for Start with currentLeader returned
// return (index, term, isLeader, currentLeader)
//
func (rf *Raft) StartWithCurrentLeader(command interface{}) (int, int, bool, int) {
	index, term, isLeader := rf.Start(command)
	currentLeader := _NA
	if isLeader {
		return index, term, isLeader, currentLeader
	} else {
		rf.Lock()
		if id, ok := rf.termLeader[term]; ok {
			currentLeader = id
		}
		rf.Unlock()
		LogRaft(vExcessive, tTrace, rf.me, "currentLeader%v", currentLeader)
		return index, term, isLeader, currentLeader
	}
}

// SEND APPLYMSG

//
// must be called with rf.Lock()
//
func (rf *Raft) sendApplyMsg(newCommitIndex int, snapshot bool) {
	oldCommitIndex := rf.commitIndex
	rf.commitIndex = newCommitIndex
	var topic LogTopic
	if snapshot {
		rf.cond.L.Lock()
		rf.applyMsgList = append(rf.applyMsgList,
			&ApplyMsg{SnapshotValid: true, Snapshot: rf.snapshot,
				SnapshotTerm: rf.log[0].Term, SnapshotIndex: rf.firstLogIndex})
		rf.cond.Signal()
		rf.cond.L.Unlock()
		topic = tSnapshot
	} else {
		// Lab Hint: Send each newly committed entry on `applyCh` on each peer.
		rf.cond.L.Lock()
		for i := oldCommitIndex + 1; i <= newCommitIndex; i++ {
			rf.applyMsgList = append(rf.applyMsgList,
				&ApplyMsg{CommandValid: true, Command: rf.getLog(i).Command, CommandIndex: i})
		}
		rf.cond.Signal()
		rf.cond.L.Unlock()
		topic = tCommit
	}

	if newCommitIndex-oldCommitIndex == 1 {
		go LogRaft(vBasic, topic, rf.me, "applyMsg %v\n", newCommitIndex)
	} else {
		go LogRaft(vBasic, topic, rf.me, "applyMsgs %v~%v\n", oldCommitIndex+1, newCommitIndex)
	}
}

//
// Raft Structure Advice: a separate long-running goroutine that sends committed log entries in order on the applyCh
// it must be a single goroutine, since otherwise it may be hard to ensure that you send log entries in log order
//
func (rf *Raft) applyMsg() {
	var applyMsg *ApplyMsg

	for !rf.killed() {
		rf.cond.L.Lock()
		if len(rf.applyMsgList) == 0 {
			rf.cond.Wait()
		}
		applyMsg, rf.applyMsgList = rf.applyMsgList[0], rf.applyMsgList[1:]
		rf.cond.L.Unlock()

		rf.applyCh <- *applyMsg
	}
}

// KILL

//
// the tester doesn't halt goroutines created by Raft after each test,
// but it does call the Kill() method. your code can use killed() to
// check whether Kill() has been called. the use of atomic avoids the
// need for a lock.
//
// the issue is that long-running goroutines use memory and may chew
// up CPU time, perhaps causing later tests to fail and generating
// confusing debug output. any goroutine with a long-running loop
// should call killed() to check whether it should stop.
//
func (rf *Raft) Kill() {
	atomic.StoreInt32(&rf.dead, 1)
	// Your code here, if desired.
}

func (rf *Raft) killed() bool {
	z := atomic.LoadInt32(&rf.dead)
	return z == 1
}

// TICKER AND ROLE HANDLER

// The ticker go routine starts a new election if this peer hasn't received
// heartsbeats recently.
func (rf *Raft) ticker() {
	i := 0
	for rf.killed() == false {

		// Your code here to check if a leader election should
		// be started and to randomize sleeping time using
		// time.Sleep().
		if i%10 == 0 {
			go LogRaft(vExcessive, tTrace, rf.me, "F round%v\n", i)
		}
		i++

		randomTimeout := randomTimeout()
		time.Sleep(randomTimeout)

		rf.Lock()
		timeout := time.Now().Sub(rf.timeoutStart)
		role := rf.role
		rf.Unlock()
		if role == rFollower && timeout > randomTimeout {
			go rf.candidate()
		}
	}
}

func (rf *Raft) candidate() {
	// Candidates: On conversion to candidate, start election
	rf.Lock()

	rf.role = rCandidate
	rf.currentTerm++
	rf.votedFor = rf.me
	rf.persist()
	rf.timeoutStart = time.Now()

	// Prepare for RequestVote
	term := rf.currentTerm
	lastLogIndex, lastLogTerm := rf.lastLogStat()

	rf.Unlock()

	LogRaft(vBasic, tCandidate, rf.me, "C in T%v (candidate)\n", term)

	// Candidates: Send RequestVote RPCs to all other servers
	voteCounter := voteCounter{n: 1}
	for i := 0; i < rf.nPeers; i++ {
		if i != rf.me {
			go rf.sendRequestVote(i, term, lastLogIndex, lastLogTerm, &voteCounter)
		}
	}

	// Wait for votes
	randomTimeout := randomTimeout()
	for rf.killed() == false {
		rf.Lock()
		role := rf.role
		timeout := time.Now().Sub(rf.timeoutStart)
		rf.Unlock()

		// Converted to follower
		//		condition1. Candidates: If AppendEntries RPC received from new leader: convert to follower
		//		other conditions
		if role == rFollower {
			break
		}

		// Candidates: If election timeout elapses: start new election
		if timeout > randomTimeout {
			go rf.candidate()
			break
		}

		// Candidates: If votes received from majority of servers: become leader
		voteCounter.Lock()
		nVote := voteCounter.n
		voteCounter.Unlock()
		if nVote > rf.nPeers/2 {
			go rf.leader()
			break
		}

		// Intervals for candidate to recheck
		time.Sleep(_LoopInterval)
	}
}

type voteCounter struct {
	sync.Mutex
	n int
}

func (rf *Raft) leader() {
	rf.Lock()
	rf.role = rLeader
	term := rf.currentTerm
	// State: Volatile state on leaders: (Reinitialized after election)
	lastLogIndex, _ := rf.lastLogStat()
	for i := 0; i < rf.nPeers; i++ {
		rf.nextIndex[i] = lastLogIndex + 1
		rf.matchIndex[i] = 0
	}
	rf.Unlock()

	// Leaders: Upon election: send initial empty AppendEntries RPCs (heartbeat) to each server;
	//		repeat during idle periods to prevent election timeouts
	rf.sendAppendEntries(term, true)

	LogRaft(vBasic, tLeader, rf.me, "C -> L/T%v! (leader)\n", term)

	matchIndex := make([]int, rf.nPeers)

	i := 0
	for rf.killed() == false {
		if i%10 == 0 {
			go LogRaft(vExcessive, tTrace, rf.me, "L round%v\n", i)
		}
		i++

		time.Sleep(_HeartbeatsInterval)

		// Not leader anymore
		rf.Lock()
		if rf.role != rLeader {
			rf.Unlock()
			LogRaft(vBasic, tDemotion, rf.me, "L -> F.. (leader)\n")
			break
		}

		// Leaders: If there exists an N such that N > commitIndex, a majority of matchIndex[i] ≥ N,
		//		and log[N].term == currentTerm: set commitIndex = N
		copy(matchIndex, rf.matchIndex)
		sort.Ints(matchIndex)
		N := matchIndex[rf.nPeers/2+1]
		if N > rf.commitIndex && rf.getLog(N).Term == rf.currentTerm {
			rf.sendApplyMsg(N, false)

			LogRaft(vBasic, tLeader, rf.me, "commit%v (leader)", rf.commitIndex)
		}
		rf.Unlock()

		// send AppendEntries RPCs periodically
		rf.sendAppendEntries(term, false)
	}
}

// MAKE

//
// the service or tester wants to create a Raft server. the ports
// of all the Raft servers (including this one) are in peers[]. this
// server's port is peers[me]. all the servers' peers[] arrays
// have the same order. persister is a place for this server to
// save its persistent state, and also initially holds the most
// recent saved state, if any. applyCh is a channel on which the
// tester or service expects Raft to send ApplyMsg messages.
// Make() must return quickly, so it should start goroutines
// for any long-running work.
//
func Make(peers []*labrpc.ClientEnd, me int,
	persister *Persister, applyCh chan ApplyMsg) *Raft {
	rf := &Raft{}
	rf.peers = peers
	rf.persister = persister
	rf.me = me

	// Your initialization code here (2A, 2B, 2C).
	go LogRaft(vExcessive, tTrace, rf.me, "new raft!\n")
	nPeers := len(peers)

	rf.currentTerm = 0
	rf.votedFor = _NA
	rf.log = []*LogEntry{{Term: 0}}

	rf.commitIndex = 0
	rf.lastApplied = 0

	rf.nextIndex = make([]int, nPeers)
	rf.matchIndex = make([]int, nPeers)

	rf.snapshot = nil
	rf.firstLogIndex = 0

	rf.nPeers = nPeers
	rf.timeoutStart = time.Now()
	rf.role = rFollower
	rf.termLeader = map[int]int{}

	rf.applyCh = applyCh
	rf.cond = sync.NewCond(&rf.condLock)
	rf.applyMsgList = make([]*ApplyMsg, 0)

	// initialize from state persisted before a crash
	rf.readPersist(persister.ReadRaftState())

	// start ticker goroutine to start elections
	go rf.ticker()

	// start sendApplyMsg goroutine
	go rf.applyMsg()

	return rf
}
