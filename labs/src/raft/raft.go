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
	"math/rand"
	//	"bytes"
	"sync"
	"sync/atomic"
	"time"

	//	"6.824/labgob"
	"6.824/labrpc"

	// additional
	. "6.824/util"
)

// TIMING

const (
	_HeartbeatsInterval = 100 * time.Millisecond
	_MinTimeout         = 10 // as number of heartbeats interval
	_MaxTimeout         = 15 // as number of heartbeats interval
)

// TYPE

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

//
// Raft log entry. It is used in RPCs. Therefore,
// all field names start with capital letters.
//
type LogEntry struct {
	Command interface{}

	Index int
	Term  int
}

//
// A Go object implementing a single Raft peer.
//
type Raft struct {
	mu        sync.Mutex          // Lock to protect shared access to this peer's state
	peers     []*labrpc.ClientEnd // RPC end points of all peers
	persister *Persister          // Object to hold this peer's persisted state
	me        int                 // this peer's index into peers[]
	dead      int32               // set by Kill()

	// Your data here (2A, 2B, 2C).
	// Look at the paper's Figure 2 for a description of what
	// state a Raft server must maintain.

	// Persistent state on all servers
	//	(Updated on stable storage before responding to RPCs)
	currentTerm int         // latest term server has seen (initialized to 0 on first boot, increases monotonically)
	votedFor    int         // candidateId that received vote in current term (or null if none)
	log         []*LogEntry // log entries; each entry contains command for state machine, and term when entry was received by leader (first index is 1)

	// Volatile state on all servers
	commitIndex int // index of highest log entry known to be committed (initialized to 0, increases monotonically)
	lastApplied int // index of highest log entry applied to state machine (initialized to 0, increases monotonically)

	// Volatile state on leaders
	nextIndex  []int // for each server, index of the next log entry to send to that server (initialized to leader last log index + 1)
	matchIndex []int // for each server, index of highest log entry known to be replicated on server (initialized to 0, increases monotonically)

	// Additional
	role            string    // this peer's current role
	electionTimeout time.Time // timeout to start election
}

// RAFT HELPER METHODS

func (rf *Raft) init(peers []*labrpc.ClientEnd, me int, persister *Persister, ch chan ApplyMsg) {
	rf.peers = peers
	rf.persister = persister
	rf.me = me

	rf.currentTerm = 0
	rf.votedFor = NA
	rf.log = make([]*LogEntry, 1)
	rf.log[0] = &LogEntry{Index: 0, Term: 0}

	rf.commitIndex = 0
	rf.lastApplied = 0

	rf.nextIndex = make([]int, rf.nPeers())
	rf.matchIndex = make([]int, rf.nPeers())

	rf.role = Follower
	rf.resetElectionTimeoutLck(true)
}

func (rf *Raft) nPeers() int {
	return len(rf.peers)
}

func (rf *Raft) resetElectionTimeoutLck(init bool) {
	randTimeout := time.Duration(rand.Int() % (_MaxTimeout - _MinTimeout))
	if init {
		rf.electionTimeout = time.Now().Add(randTimeout * _HeartbeatsInterval)
	} else {
		rf.electionTimeout = time.Now().Add((_MinTimeout + randTimeout) * _HeartbeatsInterval)
	}
}

func (rf *Raft) lastLogIndexLck() int {
	return rf.log[len(rf.log)-1].Index
}

func (rf *Raft) lastLogTermLck() int {
	return rf.log[len(rf.log)-1].Term
}

// return currentTerm and whether this server
// believes it is the leader.
func (rf *Raft) GetState() (int, bool) {
	// Your code here (2A).
	rf.mu.Lock()
	defer rf.mu.Unlock()
	return rf.currentTerm, rf.role == Leader
}

// PERSISTOR

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

}

// REQUEST VOTE RPC HANDLER

// Invoked by candidates to gather votes (§5.2).

//
// example RequestVote RPC handler.
//
func (rf *Raft) RequestVote(args *RequestVoteArgs, reply *RequestVoteReply) {
	// Your code here (2A, 2B).
	rf.mu.Lock()
	defer rf.mu.Unlock()

	// All Servers:
	//	If RPC request or response contains term T > currentTerm: set currentTerm = T, convert to follower (§5.1)
	if args.Term > rf.currentTerm {
		rf.becomeFollowerLck(args.Term)
	}

	// 5.4.1 Election restriction: If the logs have last entries with different terms,
	//	then the log with the later term is more up-to-date.
	//	If the logs end with the same term, then whichever log is longer is more up-to-date.
	upToDate := args.LastLogTerm > rf.lastLogTermLck() ||
		(args.LastLogTerm == rf.lastLogTermLck() && args.LastLogIndex >= rf.lastLogIndexLck())

	LogRaft(VBasic, TVote, rf.me, "voteRequest from %v: lastLog i%v/t%v, voteFor %v (RequestVote)",
		args.CandidateId, rf.lastLogIndexLck(), rf.lastLogTermLck(), rf.votedFor)

	reply.Term = rf.currentTerm
	// Receiver implementation:
	//	1. Reply false if term < currentTerm (§5.1)
	if args.Term < rf.currentTerm {
		reply.VoteGranted = false
		return
	}
	//	2. If votedFor is null or candidateId, and candidate’s log is
	//		at least as up-to-date as receiver’s log, grant vote (§5.2, §5.4)
	if (rf.votedFor == NA || rf.votedFor == args.CandidateId) && upToDate {
		reply.VoteGranted = true
		rf.votedFor = args.CandidateId
		// Followers (§5.2):
		//	If election timeout elapses without receiving AppendEntries RPC from
		//		current leader or granting vote to candidate: convert to candidate
		rf.resetElectionTimeoutLck(false)
	}
}

func (rf *Raft) sendRequestVote(server int, args *RequestVoteArgs, reply *RequestVoteReply) bool {
	LogRaft(VBasic, TCandidate, rf.me, "requestVote from %v: %v (sendRequestVote)", server, args)

	return rf.peers[server].Call(rpcRequestVote, args, reply)
}

// APPEND ENTRIES RPC HANDLER

// Invoked by leader to replicate log entries (§5.3); also used as heartbeat (§5.2).

func (rf *Raft) AppendEntries(args *AppendEntriesArgs, reply *AppendEntriesReply) {
	rf.mu.Lock()
	defer rf.mu.Unlock()

	// All Servers:
	//	If RPC request or response contains term T > currentTerm: set currentTerm = T, convert to follower (§5.1)
	if args.Term > rf.currentTerm {
		rf.becomeFollowerLck(args.Term)
	}

	// Candidates (§5.2):
	//	If AppendEntries RPC received from new leader: convert to follower
	if rf.role == Candidate && rf.currentTerm <= args.Term {
		rf.becomeFollowerLck(args.Term)
	}

	// Receiver implementation:
	//	1. Reply false if term < currentTerm (§5.1)
	if args.Term < rf.currentTerm {
		reply.Success = false
		return
	}

	// Followers (§5.2):
	//	If election timeout elapses without receiving AppendEntries RPC from
	//		current leader or granting vote to candidate: convert to candidate
	rf.resetElectionTimeoutLck(false)
}

func (rf *Raft) sendAppendEntries(server int, args *AppendEntriesArgs, reply *AppendEntriesReply) bool {
	LogRaft(VBasic, TLeader, rf.me, "appendEntries to %v: %v (sendAppendEntries)", server, args)

	return rf.peers[server].Call(rpcAppendEntries, args, reply)
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
	index := -1
	term := -1
	isLeader := true

	// Your code here (2B).

	return index, term, isLeader
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

// TICKER

// The ticker go routine starts a new election if this peer hasn't received
// heartsbeats recently.
func (rf *Raft) ticker() {
	for rf.killed() == false {

		// Your code here to check if a leader election should
		// be started and to randomize sleeping time using
		// time.Sleep().
		rf.tick()
		time.Sleep(_HeartbeatsInterval)
	}
}

func (rf *Raft) tick() {
	rf.mu.Lock()
	defer rf.mu.Unlock()

	LogRaft(VVerbose, TTrace, rf.me, "role: %v (tick)", rf.role)

	if rf.role == Leader {
		// Leaders:
		//	repeat (heartbeat) during idle periods to prevent election timeouts (§5.2)
		rf.heartbeatsLck()
	} else if time.Now().After(rf.electionTimeout) {
		// Followers (§5.2):
		//	If election timeout elapses without receiving AppendEntries RPC from
		//		current leader or granting vote to candidate: convert to candidate
		// Candidates (§5.2):
		//	If election timeout elapses: start new election
		rf.becomeCandidateLck()
	}
}

// FOLLOWER

func (rf *Raft) becomeFollowerLck(term int) {
	rf.role = Follower
	rf.votedFor = NA
	rf.currentTerm = term

	LogRaft(VBasic, TDemotion, rf.me, "convert to follower in term %v (becomeFollowerLck)", rf.currentTerm)
}

// CANDIDATE

func (rf *Raft) becomeCandidateLck() {
	// Candidates (§5.2):
	//	On conversion to candidate, start election:
	//		Increment currentTerm
	//		Vote for self
	//		Reset election timer
	rf.role = Candidate
	rf.currentTerm++
	rf.votedFor = rf.me
	rf.resetElectionTimeoutLck(false)

	LogRaft(VBasic, TTerm, rf.me, "candidate in term %v (becomeCandidateLck)", rf.currentTerm)

	//		Send RequestVote RPCs to all other servers
	rf.requestVotesLck()
}

func (rf *Raft) requestVotesLck() {
	args := &RequestVoteArgs{rf.currentTerm, rf.me, rf.lastLogIndexLck(), rf.lastLogTermLck()}
	votes := 1
	for i := 0; i < rf.nPeers(); i++ {
		if i != rf.me {
			go rf.requestVote(i, args, &votes)
		}
	}
}

func (rf *Raft) requestVote(server int, args *RequestVoteArgs, votes *int) {
	reply := &RequestVoteReply{}
	ok := rf.sendRequestVote(server, args, reply)

	rf.mu.Lock()
	defer rf.mu.Unlock()

	if ok && rf.currentTerm == args.Term {
		rf.processRequestVoteReplyLck(server, reply, votes)
	} else {
		LogRaft(VVerbose, TError, rf.me, "sendRequestVote to %v falied (appendEntries)", server)
	}
}

func (rf *Raft) processRequestVoteReplyLck(server int, reply *RequestVoteReply, votes *int) {
	LogRaft(VBasic, TCandidate, rf.me, "vote reply from %v in term %v: %v (processRequestVoteReplyLck)", server, rf.currentTerm, reply)

	switch reply.VoteGranted {
	case true:
		*votes++
		// Candidates (§5.2):
		//	If votes received from majority of servers: become leader
		if *votes > rf.nPeers()/2 {
			rf.becomeLeaderLck()
		}
	case false:
		// All Servers:
		//	If RPC request or response contains term T > currentTerm: set currentTerm = T, convert to follower (§5.1)
		if reply.Term > rf.currentTerm {
			rf.becomeFollowerLck(reply.Term)
		}
	}
}

// LEADER

func (rf *Raft) becomeLeaderLck() {
	LogRaft(VBasic, TLeader, rf.me, "become leader in term %v", rf.currentTerm)

	// Volatile state on leaders:
	//	(Reinitialized after election)
	for i := 0; i < rf.nPeers(); i++ {
		rf.nextIndex[i] = rf.lastLogIndexLck() + 1
		rf.matchIndex[i] = 0
	}
	// Leaders:
	//	Upon election: send initial empty AppendEntries RPCs (heartbeat) to each server (§5.2)
	rf.role = Leader
	rf.heartbeatsLck()
}

func (rf *Raft) heartbeatsLck() {
	rf.appendEntriesLck()
}

func (rf *Raft) appendEntriesLck() {
	for i := 0; i < rf.nPeers(); i++ {
		if i != rf.me {
			go rf.appendEntries(i)
		}
	}
}

func (rf *Raft) appendEntries(server int) {
	args := rf.makeAppendEntriesArgs(server)
	reply := &AppendEntriesReply{}

	ok := rf.sendAppendEntries(server, args, reply)

	rf.mu.Lock()
	defer rf.mu.Unlock()

	if ok && rf.currentTerm == args.Term {
		rf.processAppendEntriesReplyLck(reply)
	} else {
		LogRaft(VVerbose, TError, rf.me, "sendAppendEntries to %v falied (appendEntries)", server)
	}
}

func (rf *Raft) makeAppendEntriesArgs(server int) *AppendEntriesArgs {
	rf.mu.Lock()
	defer rf.mu.Unlock()

	prevLogIndex := NA
	prevLogTerm := NA
	entries := []LogEntry{}
	return &AppendEntriesArgs{rf.currentTerm, rf.me, prevLogIndex, prevLogTerm, entries, rf.commitIndex}
}

func (rf *Raft) processAppendEntriesReplyLck(reply *AppendEntriesReply) {
	LogRaft(VBasic, TLeader, rf.me, "append reply in term %v: %v (processAppendEntriesReplyLck)", rf.currentTerm, reply)
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
func Make(peers []*labrpc.ClientEnd, me int, persister *Persister, applyCh chan ApplyMsg) *Raft {
	LogRaft(VBasic, TTrace, me, "new peer (Make)")

	// initialization
	rf := &Raft{}
	rf.init(peers, me, persister, applyCh)

	// initialize from state persisted before a crash
	rf.readPersist(persister.ReadRaftState())

	// start ticker goroutine to start elections
	go rf.ticker()

	return rf
}
