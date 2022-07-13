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

const (
	NA = -1

	// log verbosity level
	LogBasic LogVerbosity = iota
	LogVerbose
	LogExcess

	// log topic
	dError      LogTopic = "ERRO"
	dLeader     LogTopic = "LEAD"
	dCandidate  LogTopic = "CAND"
	dDemotion   LogTopic = "DEMO"
	dTerm       LogTopic = "TERM"
	dLogFail    LogTopic = "LOG0"
	dLogSuccess LogTopic = "LOG1"
	dCommit     LogTopic = "CMIT"
)

// 2A UNUSED

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

	// Additional
	nPeers       int
	timeoutStart time.Time
	role         PeerRole

	// ApplyMsg
	applyCh    chan ApplyMsg
	condLock   sync.Mutex
	cond       *sync.Cond
	msgIndList []map[int]int
}

type LogEntry struct {
	Term    int
	Command interface{}
}

type PeerRole int

const (
	// peer role
	Leader PeerRole = iota
	Candidate
	Follower

	// msgIndList Tag
	OldCommitIndex = iota
	NewCommitIndex
)

// TIMING

const (
	// general loop
	LoopInterval = 10 * time.Millisecond

	// heartbeats
	HeartbeatsInterval = 100 * time.Millisecond

	// timeout
	TimeoutMin = 1000
	TimeoutMax = 1500
)

func randomTimeout() time.Duration {
	return time.Millisecond * time.Duration(TimeoutMin+rand.Intn(TimeoutMax-TimeoutMin))
}

// HELPER

//
// custom rf.Lock(), get the lock only if rf.currentTerm keeps the same and return is true
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
// return lastLogIndex, lastLogTerm.
// must be called with rf.Lock()
//
func (rf *Raft) lastLogStat() (int, int) {
	lastLogIndex := len(rf.log) - 1
	lastLogTerm := rf.log[lastLogIndex].Term
	return lastLogIndex, lastLogTerm
}

//
// All Servers: If RPC request or response contains term T > currentTerm: set currentTerm = T, convert to follower
// must be called with rf.Lock()
//
func (rf *Raft) demotion(term int) {
	rf.currentTerm = term
	rf.votedFor = NA
	rf.role = Follower
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
	return rf.currentTerm, rf.role == Leader
}

// 2A UNUSED

//
// save Raft's persistent state to stable storage,
// where it can later be retrieved after a crash and restart.
// see paper's Figure 2 for a description of what should be persistent.
//
func (rf *Raft) persist() {
	// Your code here (2C).
	// Example:
	// w := new(bytes.Buffer)
	// e := labgob.NewEncoder(w)
	// e.Encode(rf.xxx)
	// e.Encode(rf.yyy)
	// data := w.Bytes()
	// rf.persister.SaveRaftState(data)
}

//
// restore previously persisted state.
//
func (rf *Raft) readPersist(data []byte) {
	if data == nil || len(data) < 1 { // bootstrap without any state?
		return
	}
	// Your code here (2C).
	// Example:
	// r := bytes.NewBuffer(data)
	// d := labgob.NewDecoder(r)
	// var xxx
	// var yyy
	// if d.Decode(&xxx) != nil ||
	//    d.Decode(&yyy) != nil {
	//   error...
	// } else {
	//   rf.xxx = xxx
	//   rf.yyy = yyy
	// }
}

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

	// Additional
	VoterId int
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
		go LogRaft(LogBasic, dTerm, rf.me, "F in T%v (RequestVote)/C%v\n", rf.currentTerm, args.CandidateId)
	}

	reply.Term = rf.currentTerm
	reply.VoterId = rf.me

	lastLogIndex, lastLogTerm := rf.lastLogStat()

	// Receiver implementation: Reply false if term < currentTerm
	if args.Term >= rf.currentTerm &&
		// Receiver implementation: If votedFor is null or candidateId,
		(rf.votedFor == NA || rf.votedFor == args.CandidateId) &&
		//		and candidate’s log is at least as up-to-date as receiver’s log, grant vote
		//		5.4.1 Election restriction: If the logs have last entries with different terms,
		//			then the log with the later term is more up-to-date.
		//			If the logs end with the same term, then whichever log is longer is more up-to-date.
		(args.LastLogTerm > lastLogTerm ||
			(args.LastLogTerm == lastLogTerm && args.LastLogIndex >= lastLogIndex)) {

		rf.votedFor = args.CandidateId
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
func (rf *Raft) sendRequestVote(server int, term int, lastLogIndex int, lastLogTerm int, voteCounter *VoteCounter) {
	args := &RequestVoteArgs{term, rf.me, lastLogIndex, lastLogTerm}
	reply := &RequestVoteReply{}

	ok := rf.peers[server].Call("Raft.RequestVote", args, reply)

	if !rf.termLock(term) {
		return
	}

	switch true {
	case !ok:
		defer LogRaft(LogVerbose, dError, rf.me, "C send RequestVote to V#%v failed!\n", server)
	case reply.Term > rf.currentTerm:
		// All Servers: If RPC request or response contains term T > currentTerm:
		//		set currentTerm = T, convert to follower
		rf.demotion(reply.Term)
		defer LogRaft(LogBasic, dTerm, rf.me, "C->F in T%v/S%v (sendRequestVote)\n", reply.Term, reply.VoterId)
	case reply.VoteGranted == true:
		defer func() {
			voteCounter.Lock()
			voteCounter.n += 1
			voteCounter.Unlock()
			LogRaft(LogBasic, dCandidate, rf.me, "C votedBy V%v! (sendRequestVote)\n", server)
		}()
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

	// Additional
	ReceiverId int
}

type Inconsistency struct {
	// mismatch
	Term           int
	TermFirstIndex int
	// no entry
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
	reply.ReceiverId = rf.me

	// Candidates: If AppendEntries RPC received from new leader: convert to follower
	if rf.role == Candidate && rf.currentTerm <= args.Term {
		rf.role = Follower

		go LogRaft(LogBasic, dDemotion, rf.me, "C -> F.. (AppendEntries)/L%v\n", args.LeaderId)
	}

	// Followers: reset election timeout if receiving AppendEntries RPC from current leader
	rf.timeoutStart = time.Now()

	// All Servers: If RPC request or response contains term T > currentTerm: set currentTerm = T, convert to follower
	if args.Term > rf.currentTerm {
		rf.demotion(args.Term)

		go LogRaft(LogBasic, dTerm, rf.me, "F in T%v (AppendEntries)/L%v\n", rf.currentTerm, args.LeaderId)
	}

	// Receiver implementation 1: Reply false if term < currentTerm
	if args.Term < rf.currentTerm {
		reply.Success = false
		return
	}
	// Receiver implementation 2: Reply false if log doesn’t contain an entry
	//		at prevLogIndex whose term matches prevLogTerm
	lastLogIndex := len(rf.log) - 1
	if lastLogIndex < args.PrevLogIndex {
		reply.Success = false
		reply.Inconsistency = Inconsistency{LastLogIndex: lastLogIndex, NoEntry: true}
		return
	}
	prevLogTerm := rf.log[args.PrevLogIndex].Term
	if prevLogTerm != args.PrevLogTerm {
		reply.Success = false
		termFirstIndex := args.PrevLogIndex
		for {
			termFirstIndex--
			if rf.log[termFirstIndex].Term != prevLogTerm {
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
				if entry.Term != rf.log[index].Term {
					rf.log = append(rf.log[:index], args.Entries[i:]...)
					break
				}
			} else {
				rf.log = append(rf.log, args.Entries[i:]...)
				break
			}
		}
		go LogRaft(LogBasic, dLogSuccess, rf.me, "log%v <- L%v! (AppendEntries)", len(rf.log)-1, args.LeaderId)
	}
	// Receiver implementation 5: If leaderCommit > commitIndex,
	//		set commitIndex = min(leaderCommit, index of last new entry)
	if args.LeaderCommit > rf.commitIndex {
		oldCommitIndex := rf.commitIndex
		rf.commitIndex = min(args.LeaderCommit, lastLogIndex)
		// Lab Hint: Send each newly committed entry on `applyCh` on each peer.
		rf.cond.L.Lock()
		rf.msgIndList = append(rf.msgIndList,
			map[int]int{OldCommitIndex: oldCommitIndex, NewCommitIndex: rf.commitIndex})
		rf.cond.Signal()
		rf.cond.L.Unlock()

		go LogRaft(LogBasic, dCommit, rf.me, "commit%v <- L%v! (AppendEntries)", rf.commitIndex, args.LeaderId)
	}
}

//
// send a AppendEntries RPC to a server and process the reply
//
func (rf *Raft) sendAppendEntries(server int, term int, initial bool) {
	if !rf.termLock(term) {
		return
	}
	lastLogIndex := len(rf.log) - 1
	nextIndex := rf.nextIndex[server]
	prevLogIndex := nextIndex - 1
	prevLogTerm := rf.log[prevLogIndex].Term
	leaderCommit := rf.commitIndex
	// Leaders: If last log index ≥ nextIndex for a follower:
	//		send AppendEntries RPC with log entries starting at nextIndex
	entries := make([]*LogEntry, 0)
	if !initial && lastLogIndex >= nextIndex {
		entries = append(entries, rf.log[nextIndex:]...)
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
		defer LogRaft(LogVerbose, dError, rf.me, "L/T%v send AppendEntries to F%v failed!\n", term, server)
	case reply.Success:
		// Leaders: If successful: update nextIndex and matchIndex for follower
		rf.nextIndex[server] = lastLogIndex + 1
		rf.matchIndex[server] = lastLogIndex
	case reply.Term > rf.currentTerm:
		// All Servers: If RPC request or response contains term T > currentTerm: set currentTerm = T, convert to follower
		rf.demotion(reply.Term)
		defer LogRaft(LogBasic, dTerm, rf.me, "L->F in T%v/S%v (sendAppendEntries)\n", reply.Term, reply.ReceiverId)
	case reply.Inconsistency.NoEntry:
		// Leaders: If AppendEntries fails because of log inconsistency: decrement nextIndex and retry
		rf.nextIndex[server] = reply.Inconsistency.LastLogIndex + 1
		defer LogRaft(LogBasic, dLogFail, rf.me, "L: log%v empty/F%v (sendAppendEntries)\n", prevLogIndex, reply.ReceiverId)
	case reply.Inconsistency.Term != 0:
		// Leaders: If AppendEntries fails because of log inconsistency: decrement nextIndex and retry
		if reply.Inconsistency.Term > prevLogTerm ||
			rf.log[reply.Inconsistency.TermFirstIndex].Term != reply.Inconsistency.Term {
			rf.nextIndex[server] = reply.Inconsistency.TermFirstIndex
			defer LogRaft(LogBasic, dLogFail, rf.me, "L: log%v mismatch1/F%v (sendAppendEntries)\n", prevLogIndex, reply.ReceiverId)
		} else {
			firstIndex := reply.Inconsistency.TermFirstIndex
			for {
				firstIndex++
				if rf.log[firstIndex].Term != reply.Inconsistency.Term {
					break
				}
			}
			rf.nextIndex[server] = firstIndex
			defer LogRaft(LogBasic, dLogFail, rf.me, "L: log%v mismatch2/F%v (sendAppendEntries)\n", prevLogIndex, reply.ReceiverId)
		}
	default:
		log.Fatalln("log failed with unkown reason!")
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
	if rf.role == Leader {
		// Leader: If command received from client: append entry to local log
		defer rf.appendCommand(command)
		return len(rf.log), rf.currentTerm, true
	} else {
		return NA, NA, false
	}
}

//
// must be called with rf lock
//
func (rf *Raft) appendCommand(command interface{}) {
	logEntry := LogEntry{rf.currentTerm, command}
	rf.log = append(rf.log, &logEntry)

	go LogRaft(LogBasic, dLeader, rf.me, "log%v appendCommand", len(rf.log)-1)
}

// SEND APPLYMSG

//
// Raft Structure Advice: a separate long-running goroutine that sends committed log entries in order on the applyCh
// it must be a single goroutine, since otherwise it may be hard to ensure that you send log entries in log order
//
func (rf *Raft) sendApplyMsgs() {
	var msgInd map[int]int
	var applyMsg ApplyMsg
	for !rf.killed() {
		rf.cond.L.Lock()
		if len(rf.msgIndList) == 0 {
			rf.cond.Wait()
		}
		msgInd, rf.msgIndList = rf.msgIndList[0], rf.msgIndList[1:]
		rf.cond.L.Unlock()

		for i := msgInd[OldCommitIndex] + 1; i <= msgInd[NewCommitIndex]; i++ {
			rf.Lock()
			applyMsg = ApplyMsg{CommandValid: true, Command: rf.log[i].Command, CommandIndex: i}
			rf.Unlock()
			rf.applyCh <- applyMsg
		}
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
	for rf.killed() == false {

		// Your code here to check if a leader election should
		// be started and to randomize sleeping time using
		// time.Sleep().
		randomTimeout := randomTimeout()
		time.Sleep(randomTimeout)

		rf.Lock()
		timeout := time.Now().Sub(rf.timeoutStart)
		role := rf.role
		rf.Unlock()
		if role == Follower && timeout > randomTimeout {
			go rf.candidate()
		}
	}
}

type VoteCounter struct {
	sync.Mutex
	n int
}

func (rf *Raft) candidate() {
	// Candidates: On conversion to candidate, start election
	rf.Lock()

	rf.role = Candidate
	rf.currentTerm += 1
	rf.votedFor = rf.me
	rf.timeoutStart = time.Now()

	// Prepare for RequestVote
	term := rf.currentTerm
	lastLogIndex, lastLogTerm := rf.lastLogStat()

	rf.Unlock()

	LogRaft(LogBasic, dCandidate, rf.me, "C in T%v (candidate)\n", rf.currentTerm)

	// Candidates: Send RequestVote RPCs to all other servers
	voteCounter := VoteCounter{n: 1}
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
		if role == Follower {
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
		time.Sleep(LoopInterval)
	}
}

func (rf *Raft) leader() {
	rf.Lock()
	rf.role = Leader
	term := rf.currentTerm
	// State: Volatile state on leaders: (Reinitialized after election)
	for i := 0; i < rf.nPeers; i++ {
		rf.nextIndex[i] = len(rf.log)
		rf.matchIndex[i] = 0
	}
	rf.Unlock()

	// Leaders: Upon election: send initial empty AppendEntries RPCs (heartbeat) to each server;
	//		repeat during idle periods to prevent election timeouts
	rf._sendAppendEntries(term, true)

	LogRaft(LogBasic, dLeader, rf.me, "C -> L/T%v! (leader)\n", term)

	matchIndex := make([]int, rf.nPeers)

	for rf.killed() == false {
		time.Sleep(HeartbeatsInterval)

		// Not leader anymore
		rf.Lock()
		if rf.role != Leader {
			rf.Unlock()
			LogRaft(LogBasic, dDemotion, rf.me, "L -> F.. (leader)\n")
			break
		}

		// Leaders: If there exists an N such that N > commitIndex, a majority of matchIndex[i] ≥ N,
		//		and log[N].term == currentTerm: set commitIndex = N
		copy(matchIndex, rf.matchIndex)
		sort.Ints(matchIndex)
		N := matchIndex[rf.nPeers/2+1]
		if N > rf.commitIndex && rf.log[N].Term == rf.currentTerm {
			oldCommitIndex := rf.commitIndex
			rf.commitIndex = N

			LogRaft(LogBasic, dLeader, rf.me, "commit%v (leader)", rf.commitIndex)
			// Lab Hint: Send each newly committed entry on `applyCh` on each peer.
			rf.cond.L.Lock()
			rf.msgIndList = append(rf.msgIndList,
				map[int]int{OldCommitIndex: oldCommitIndex, NewCommitIndex: rf.commitIndex})
			rf.cond.Signal()
			rf.cond.L.Unlock()
		}
		rf.Unlock()

		// send AppendEntries RPCs periodically
		rf._sendAppendEntries(term, false)
	}
}

func (rf *Raft) _sendAppendEntries(term int, initial bool) {
	for i := 0; i < rf.nPeers; i++ {
		if i != rf.me {
			go rf.sendAppendEntries(i, term, initial)
		}
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
	nPeers := len(peers)

	rf.currentTerm = 0
	rf.votedFor = NA
	rf.log = []*LogEntry{{Term: 0}}

	rf.commitIndex = 0
	rf.lastApplied = 0

	rf.nextIndex = make([]int, nPeers)
	rf.matchIndex = make([]int, nPeers)

	rf.nPeers = nPeers
	rf.timeoutStart = time.Now()
	rf.role = Follower

	rf.applyCh = applyCh
	rf.cond = sync.NewCond(&rf.condLock)
	rf.msgIndList = make([]map[int]int, 0)

	// initialize from state persisted before a crash
	rf.readPersist(persister.ReadRaftState())

	// start ticker goroutine to start elections
	go rf.ticker()

	// start sendApplyMsg goroutine
	go rf.sendApplyMsgs()

	return rf
}
