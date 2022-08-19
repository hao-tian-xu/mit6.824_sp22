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
	"fmt"
	"log"
	"math/rand"
	"sort"

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
	_HeartbeatsInterval = 50 * time.Millisecond
	_MinTimeout         = 8  // as number of heartbeats interval
	_MaxTimeout         = 16 // as number of heartbeats interval
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

func (l LogEntry) String() string {
	return fmt.Sprintf("i%v/T%02d", l.Index, l.Term)
}

type Snapshot struct {
	LastIncludedIndex int
	LastIncludedTerm  int
	Data              []byte
}

// RAFT

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
	currentTerm int        // latest term server has seen (initialized to 0 on first boot, increases monotonically)
	votedFor    int        // candidateId that received vote in current term (or null if none)
	log         []LogEntry // log entries; each entry contains command for state machine, and term when entry was received by leader (first index is 1)
	snapshot    Snapshot

	// Volatile state on all servers
	commitIndex int // index of highest log entry known to be committed (initialized to 0, increases monotonically)
	lastApplied int // index of highest log entry applied to state machine (initialized to 0, increases monotonically)

	// Volatile state on leaders
	nextIndex  []int // for each server, index of the next log entry to send to that server (initialized to leader last log index + 1)
	matchIndex []int // for each server, index of highest log entry known to be replicated on server (initialized to 0, increases monotonically)

	// Additional
	role            string    // this peer's current role
	electionTimeout time.Time // timeout to start election
	applyCond       sync.Cond
	applySnapshot   bool
}

// INTERFACE

//
// return currentTerm and whether this server
// believes it is the leader.
//
func (rf *Raft) GetState() (int, bool) {
	// Your code here (2A).
	LogRaft(VExcessive, TTrace, rf.me, NA, "lock acquired (GetState)")
	rf.mu.Lock()
	defer LogRaft(VExcessive, TTrace, rf.me, NA, "lock released (GetState)")
	defer rf.mu.Unlock()

	return rf.currentTerm, rf.role == Leader
}

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
	LogRaft(VExcessive, TTrace, rf.me, NA, "lock acquired (Start)")
	rf.mu.Lock()
	defer LogRaft(VExcessive, TTrace, rf.me, NA, "lock released (Start)")
	defer rf.mu.Unlock()

	isLeader := rf.role == Leader
	if !isLeader {
		return NA, NA, isLeader
	}

	index := rf.lastLogL().Index + 1
	term := rf.currentTerm

	logEntry := LogEntry{command, index, term}

	rf.logL(VBasic, TClient, "append log: %v (Start)", logEntry)

	// Leaders:
	//	If command received from client: append entry to local log
	rf.log = append(rf.log, logEntry)
	rf.persistL()
	rf.appendsL()

	return index, term, isLeader
}

// Snapshot

//
// the service says it has created a snapshot that has
// all info up to and including index. this means the
// service no longer needs the log through (and including)
// that index. Raft should now trim its log as much as possible.
//
func (rf *Raft) Snapshot(index int, snapshot []byte) {
	// Your code here (2D).
	LogRaft(VExcessive, TTrace, rf.me, NA, "lock acquired (Snapshot)")
	rf.mu.Lock()
	defer LogRaft(VExcessive, TTrace, rf.me, NA, "lock released (Snapshot)")
	defer rf.mu.Unlock()

	rf.logL(VBasic, TClient, "snapshot at %v", index)

	rf.trimLogL(index)
	rf.snapshot = Snapshot{rf.log[0].Index, rf.log[0].Term, snapshot}
	rf.persistL()
}

//
// A service wants to switch to snapshot.  Only do so if Raft hasn't
// have more recent info since it communicate the snapshot on applyCh.
//
func (rf *Raft) CondInstallSnapshot(lastIncludedTerm int, lastIncludedIndex int, snapshot []byte) bool {

	// Your code here (2D).

	return true
}

// HELPER METHODS

func (rf *Raft) initL(peers []*labrpc.ClientEnd, me int, persister *Persister, ch chan ApplyMsg) {
	rf.peers = peers
	rf.persister = persister
	rf.me = me

	rf.currentTerm = 0
	rf.votedFor = NA
	rf.log = make([]LogEntry, 1)
	rf.log[0] = LogEntry{Index: 0, Term: 0}
	rf.snapshot = Snapshot{}

	rf.commitIndex = 0
	rf.lastApplied = 0

	rf.nextIndex = make([]int, rf.nPeers())
	rf.matchIndex = make([]int, rf.nPeers())

	rf.role = Follower
	rf.resetElectionTimeoutL(true)
	rf.applyCond = *sync.NewCond(&rf.mu)
}

func (rf *Raft) nPeers() int {
	return len(rf.peers)
}

func (rf *Raft) resetElectionTimeoutL(init bool) {
	randTimeout := time.Duration(rand.Int() % (_MaxTimeout - _MinTimeout))
	if init {
		rf.electionTimeout = time.Now().Add((randTimeout) * _HeartbeatsInterval)
	} else {
		rf.electionTimeout = time.Now().Add((_MinTimeout + randTimeout) * _HeartbeatsInterval)
	}
}

func (rf *Raft) lastLogL() LogEntry {
	return rf.log[len(rf.log)-1]
}

// custom log function

func (rf *Raft) logL(verbosity LogVerbosity, topic LogTopic, format string, a ...interface{}) {
	LogRaft(verbosity, topic, rf.me, rf.currentTerm, format, a...)
}

// snapshot

func (rf *Raft) getLogL(index int) LogEntry {
	return rf.log[rf.logSliceIndexL(index)]
}

func (rf *Raft) logSliceIndexL(index int) int {
	if index < rf.logFirstIndexL() {
		return NA
	} else {
		return index - rf.logFirstIndexL()
	}
}

func (rf *Raft) logFirstIndexL() int {
	return rf.log[0].Index
}

func (rf *Raft) trimLogL(index int) {
	newLog := make([]LogEntry, rf.lastLogL().Index-index+1)
	copy(newLog, rf.log[index-rf.logFirstIndexL():])
	rf.log = newLog
}

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

// RequestVote RPC Handler

//
// Invoked by candidates to gather votes (§5.2).
//
func (rf *Raft) RequestVote(args *RequestVoteArgs, reply *RequestVoteReply) {
	// Your code here (2A, 2B).
	LogRaft(VExcessive, TTrace, rf.me, NA, "lock acquired (RequestVote)")
	rf.mu.Lock()
	defer LogRaft(VExcessive, TTrace, rf.me, NA, "lock released (RequestVote)")
	defer rf.mu.Unlock()

	// All Servers:
	//	If RPC request or response contains term T > currentTerm: set currentTerm = T, convert to follower (§5.1)
	if args.Term > rf.currentTerm {
		rf.becomeFollowerL(args.Term)
	}

	// 5.4.1 Election restriction: If the logs have last entries with different terms,
	//	then the log with the later term is more up-to-date.
	//	If the logs end with the same term, then whichever log is longer is more up-to-date.
	upToDate := args.LastLogTerm > rf.lastLogL().Term ||
		(args.LastLogTerm == rf.lastLogL().Term && args.LastLogIndex >= rf.lastLogL().Index)

	rf.logL(VBasic, TVote, "request from %v: voteFor %v, lastLog %v, upToDate %v (RequestVote)",
		args.CandidateId, rf.votedFor, rf.lastLogL(), upToDate)

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
		rf.persistL()
		// Followers (§5.2):
		//	If election timeout elapses without receiving AppendEntries RPC from
		//		current leader or granting vote to candidate: convert to candidate
		rf.resetElectionTimeoutL(false)
	}
}

func (rf *Raft) sendRequestVote(server int, args *RequestVoteArgs, reply *RequestVoteReply) bool {
	return rf.peers[server].Call(rpcRequestVote, args, reply)
}

// AppendEntries RPC Handler

//
// Invoked by leader to replicate log entries (§5.3); also used as heartbeat (§5.2).
//
func (rf *Raft) AppendEntries(args *AppendEntriesArgs, reply *AppendEntriesReply) {
	LogRaft(VExcessive, TTrace, rf.me, NA, "lock acquired (AppendEntries)")
	rf.mu.Lock()
	defer LogRaft(VExcessive, TTrace, rf.me, NA, "lock released (AppendEntries)")
	defer rf.mu.Unlock()

	// All Servers:
	//	If RPC request or response contains term T > currentTerm: set currentTerm = T, convert to follower (§5.1)
	if args.Term > rf.currentTerm {
		rf.becomeFollowerL(args.Term)
	}

	// Candidates (§5.2):
	//	If AppendEntries RPC received from new leader: convert to follower
	if rf.role == Candidate && args.Term >= rf.currentTerm {
		rf.becomeFollowerL(args.Term)
	}

	// Followers (§5.2):
	//	If election timeout elapses without receiving AppendEntries RPC from
	//		current leader or granting vote to candidate: convert to candidate
	if args.Term >= rf.currentTerm {
		rf.resetElectionTimeoutL(false)
	}

	rf._processAppendEntriesL(args, reply)
}

func (rf *Raft) _processAppendEntriesL(args *AppendEntriesArgs, reply *AppendEntriesReply) {
	rf.logL(VBasic, TAppend, "append from %v: lastLog %v (%v) (_processAppendEntriesL)",
		args.LeaderId, rf.lastLogL(), args)

	reply.Term = rf.currentTerm
	reply.Success = true

	// Receiver implementation:
	//	1. Reply false if term < currentTerm (§5.1)
	if args.Term < rf.currentTerm {
		reply.Success = false
		return
	}
	// Receiver implementation:
	//	2. Reply false if log doesn’t contain an entry at prevLogIndex whose term matches prevLogTerm (§5.3)
	if rf._prevLogConflictL(args, reply) {
		reply.Success = false
		return
	}
	//	3. If an existing entry conflicts with a new one (same index but different terms),
	//		delete the existing entry and all that follow it (§5.3)
	//	4. Append any new entries not already in the log
	for i, entry := range args.Entries {
		index := args.PrevLogIndex + 1 + i
		if rf.lastLogL().Index < index || entry.Term != rf.getLogL(index).Term {
			rf.log = append(rf.log[:rf.logSliceIndexL(index)], args.Entries[i:]...)
			rf.persistL()
			break
		}
	}
	//	5. If leaderCommit > commitIndex, set commitIndex = min(leaderCommit, index of last new entry)
	if args.LeaderCommit > rf.commitIndex {
		rf.commitIndex = Min(args.LeaderCommit, rf.lastLogL().Index)

		rf.logL(VBasic, TCommit, "commit %v (AppendEntries)", rf.commitIndex)

		// All Servers:
		//	If commitIndex > lastApplied: increment lastApplied, apply log[lastApplied] to state machine (§5.3)
		rf.applySignalL()
	}
}

func (rf *Raft) _prevLogConflictL(args *AppendEntriesArgs, reply *AppendEntriesReply) bool {
	if rf.lastLogL().Index < args.PrevLogIndex {
		// log doesn’t contain an entry at prevLogIndex
		reply.ConflictTerm = NA
		reply.ConflictFirstIndex = rf.lastLogL().Index + 1
		return true
	} else if rf.logFirstIndexL() > args.PrevLogIndex {
		// Snapshot Additional: log is trimmed beyond prevLogIndex
		reply.ConflictTerm = NA
		reply.ConflictFirstIndex = rf.logFirstIndexL() + 1
		return true
	} else if rf.getLogL(args.PrevLogIndex).Term != args.PrevLogTerm {
		// term of log's entry at prevLogIndex doesn't matches prevLogTerm
		reply.ConflictTerm = rf.getLogL(args.PrevLogIndex).Term
		index := args.PrevLogIndex - 1
		for index > rf.logFirstIndexL() {
			if rf.getLogL(index).Term != reply.ConflictTerm {
				index++
				break
			}
			index--
		}
		reply.ConflictFirstIndex = index
		return true
	}
	return false
}

func (rf *Raft) sendAppendEntries(server int, args *AppendEntriesArgs, reply *AppendEntriesReply) bool {
	return rf.peers[server].Call(rpcAppendEntries, args, reply)
}

// Snapshot RPC Handler

//
// Invoked by leader to send a snapshot to a follower.
//
func (rf *Raft) InstallSnapshot(args *InstallSnapshotArgs, reply *InstallSnapshotReply) {
	LogRaft(VExcessive, TTrace, rf.me, NA, "lock acquired (InstallSnapshot)")
	rf.mu.Lock()
	defer LogRaft(VExcessive, TTrace, rf.me, NA, "lock released (InstallSnapshot)")
	defer rf.mu.Unlock()

	// All Servers:
	//	If RPC request or response contains term T > currentTerm: set currentTerm = T, convert to follower (§5.1)
	if args.Term > rf.currentTerm {
		rf.becomeFollowerL(args.Term)
	}

	rf.logL(VBasic, TSnapshot, "install from %v (%v) (InstallSnapshot)",
		args.LeaderId, args)

	reply.Term = rf.currentTerm

	// Receiver implementation:
	//	1. Reply immediately if term < currentTerm
	if args.Term < rf.currentTerm {
		return
	}
	//	5. Save snapshot file, discard any existing or partial snapshot with a smaller index
	if rf.logFirstIndexL() >= args.Snapshot.LastIncludedIndex {
		return
	}
	rf.snapshot = args.Snapshot
	//	6. If existing log entry has same index and term as snapshot’s last included entry,
	//		retain log entries following it and reply
	//	7. Discard the entire log
	if rf.lastLogL().Index >= args.Snapshot.LastIncludedIndex &&
		rf.getLogL(args.Snapshot.LastIncludedIndex).Term == args.Snapshot.LastIncludedTerm {
		rf.trimLogL(args.Snapshot.LastIncludedIndex)
	} else {
		rf.log = []LogEntry{{Index: args.Snapshot.LastIncludedIndex, Term: args.Snapshot.LastIncludedTerm}}
	}
	//	8. Reset state machine using snapshot contents
	rf.applySnapshot = true
	rf.applySignalL()
}

func (rf *Raft) sendInstallSnapshot(server int, args *InstallSnapshotArgs, reply *InstallSnapshotReply) bool {
	return rf.peers[server].Call(rpcInstallSnapshot, args, reply)
}

// PERSISTOR

//
// save Raft's persistent state to stable storage,
// where it can later be retrieved after a crash and restart.
// see paper's Figure 2 for a description of what should be persistent.
//
func (rf *Raft) persistL() {
	// rf.persister.SaveRaftState(data)
	w := new(bytes.Buffer)
	e := labgob.NewEncoder(w)
	if e.Encode(rf.currentTerm) != nil ||
		e.Encode(rf.votedFor) != nil ||
		e.Encode(rf.log) != nil {

		log.Fatalln("persistL() failed!")
	} else {
		data := w.Bytes()
		rf.persister.SaveStateAndSnapshot(data, rf.snapshot.Data)
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
	var _log []LogEntry
	if d.Decode(&currentTerm) != nil ||
		d.Decode(&votedFor) != nil ||
		d.Decode(&_log) != nil {

		log.Fatalln("readPersist() failed!")
	} else {
		rf.currentTerm = currentTerm
		rf.votedFor = votedFor
		rf.log = _log
		rf.snapshot = Snapshot{rf.log[0].Index, rf.log[0].Term, rf.persister.ReadSnapshot()}

		rf.commitIndex = rf.logFirstIndexL()
		rf.lastApplied = rf.logFirstIndexL()
	}
}

// APPLIER

func (rf *Raft) apply(applyCh chan ApplyMsg) {
	LogRaft(VExcessive, TTrace, rf.me, NA, "lock acquired (apply)")
	rf.mu.Lock()
	defer LogRaft(VExcessive, TTrace, rf.me, NA, "lock released (apply)")
	defer rf.mu.Unlock()

	for !rf.killed() {
		// InstallSnapshot RPC Receiver implementation:
		//	8. Reset state machine using snapshot contents
		if rf.applySnapshot {
			rf.applySnapshotL(applyCh)
		}

		// All Servers:
		//	If commitIndex > lastApplied: increment lastApplied, apply log[lastApplied] to state machine (§5.3)
		if rf.commitIndex > rf.lastApplied {
			rf.lastApplied++

			rf.logL(VBasic, TApply, "apply log %v (apply)", rf.lastApplied)

			//rf.logL(VTemp, TTrace, "lastApplied %v, firstIndex %v", rf.lastApplied, rf.logFirstIndexL())

			applyMsg := ApplyMsg{
				CommandValid: true,
				Command:      rf.getLogL(rf.lastApplied).Command,
				CommandIndex: rf.lastApplied,
			}

			rf.mu.Unlock()
			LogRaft(VExcessive, TTrace, rf.me, NA, "lock released (apply)")
			applyCh <- applyMsg
			LogRaft(VExcessive, TTrace, rf.me, NA, "lock acquired (apply)")
			rf.mu.Lock()
		}

		if rf.commitIndex <= rf.lastApplied && !rf.applySnapshot {
			LogRaft(VExcessive, TTrace, rf.me, NA, "lock released (apply-wait)")
			LogRaft(VExcessive, TTrace, rf.me, NA, "lock acquired (apply-wait)")
			rf.applyCond.Wait()
		}
	}
}

func (rf *Raft) applySnapshotL(applyCh chan ApplyMsg) {
	rf.applySnapshot = false

	// 2D: Take care that these snapshots only advance the service's state,
	//	and don't cause it to move backwards.
	if rf.snapshot.LastIncludedIndex > rf.lastApplied {
		rf.commitIndex = rf.snapshot.LastIncludedIndex
		rf.lastApplied = rf.snapshot.LastIncludedIndex

		rf.logL(VBasic, TApply, "apply snapshot %v (apply)", rf.lastApplied)

		applyMsg := ApplyMsg{
			SnapshotValid: true,
			Snapshot:      rf.snapshot.Data,
			SnapshotTerm:  rf.snapshot.LastIncludedIndex,
			SnapshotIndex: rf.snapshot.LastIncludedIndex,
		}

		rf.mu.Unlock()
		LogRaft(VExcessive, TTrace, rf.me, NA, "lock released (applySnapshotL)")
		applyCh <- applyMsg
		LogRaft(VExcessive, TTrace, rf.me, NA, "lock acquired (applySnapshotL)")
		rf.mu.Lock()
	}
}

func (rf *Raft) applySignalL() {
	rf.applyCond.Signal()
}

// TICKER

//
// The ticker go routine starts a new election if this peer hasn't received
// heartsbeats recently.
//
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
	LogRaft(VExcessive, TTrace, rf.me, NA, "lock acquired (tick)")
	rf.mu.Lock()
	defer LogRaft(VExcessive, TTrace, rf.me, NA, "lock released (tick)")
	defer rf.mu.Unlock()

	rf.logL(VVerbose, TTick, "role: %v (tick)", rf.role)

	if rf.role == Leader {
		// Leaders:
		//	repeat (heartbeat) during idle periods to prevent election timeouts (§5.2)
		rf.heartbeatsL()
	} else if time.Now().After(rf.electionTimeout) {
		// Followers (§5.2):
		//	If election timeout elapses without receiving AppendEntries RPC from
		//		current leader or granting vote to candidate: convert to candidate
		// Candidates (§5.2):
		//	If election timeout elapses: start new election
		rf.becomeCandidateL()
	}
}

// FOLLOWER

func (rf *Raft) becomeFollowerL(term int) {
	rf.logL(VBasic, TDemotion, "follower in T%02d (becomeFollowerL)", term)

	rf.role = Follower
	rf.votedFor = NA
	rf.currentTerm = term
	rf.persistL()
}

// CANDIDATE

func (rf *Raft) becomeCandidateL() {
	// Candidates (§5.2):
	//	On conversion to candidate, start election:
	//		Increment currentTerm
	//		Vote for self
	//		Reset election timer
	rf.role = Candidate
	rf.currentTerm++
	rf.votedFor = rf.me
	rf.persistL()
	rf.resetElectionTimeoutL(false)

	rf.logL(VBasic, TCandidate, "candidate (becomeCandidateL)")

	//		Send RequestVote RPCs to all other servers
	rf.requestsL()
}

func (rf *Raft) requestsL() {
	args := &RequestVoteArgs{rf.currentTerm, rf.me, rf.lastLogL().Index, rf.lastLogL().Term}
	votes := 1
	for i := 0; i < rf.nPeers(); i++ {
		if i != rf.me {
			go rf.request(i, args, &votes)
		}
	}
}

func (rf *Raft) request(server int, args *RequestVoteArgs, votes *int) {
	LogRaft(VBasic, TCandidate, rf.me, args.Term, "request to %v: %v (request)", server, args)

	reply := &RequestVoteReply{}
	ok := rf.sendRequestVote(server, args, reply)

	LogRaft(VExcessive, TTrace, rf.me, NA, "lock acquired (request)")
	rf.mu.Lock()
	defer LogRaft(VExcessive, TTrace, rf.me, NA, "lock released (request)")
	defer rf.mu.Unlock()

	if ok {
		rf.processRequestReplyL(server, args, reply, votes)
	} else {
		rf.logL(VVerbose, TError, "request to %v falied (request)", server)
	}
}

func (rf *Raft) processRequestReplyL(server int, args *RequestVoteArgs, reply *RequestVoteReply, votes *int) {
	if rf.currentTerm == args.Term && rf.role == Candidate {

		rf.logL(VBasic, TVote, "vote reply from %v: %v (processRequestReplyL)", server, reply)

		switch reply.VoteGranted {
		case true:
			*votes++
			// Candidates (§5.2):
			//	If votes received from majority of servers: become leader
			if *votes > rf.nPeers()/2 {
				rf.becomeLeaderL()
			}
		case false:
			// All Servers:
			//	If RPC request or response contains term T > currentTerm: set currentTerm = T, convert to follower (§5.1)
			if reply.Term > rf.currentTerm {
				rf.becomeFollowerL(reply.Term)
			}
		}
	}
}

// LEADER

func (rf *Raft) becomeLeaderL() {
	rf.logL(VBasic, TLeader, "leader (becomeLeaderL)")

	// Volatile state on leaders:
	//	(Reinitialized after election)
	for i := 0; i < rf.nPeers(); i++ {
		rf.nextIndex[i] = rf.lastLogL().Index + 1
		rf.matchIndex[i] = 0
	}
	// Leaders:
	//	Upon election: send initial empty AppendEntries RPCs (heartbeat) to each server (§5.2)
	rf.role = Leader
	rf.heartbeatsL()
}

// append entries

func (rf *Raft) heartbeatsL() {
	rf.appendsL()
}

func (rf *Raft) appendsL() {
	for i := 0; i < rf.nPeers(); i++ {
		if i != rf.me {
			go rf.append(i, rf.currentTerm)
		}
	}
}

func (rf *Raft) append(server int, term int) {
	LogRaft(VExcessive, TTrace, rf.me, NA, "lock acquired (append)")
	rf.mu.Lock()
	defer LogRaft(VExcessive, TTrace, rf.me, NA, "lock released (append)")
	defer rf.mu.Unlock()

	if rf.currentTerm != term {
		return
	}

	// 2D Hint: have the leader send an InstallSnapshot RPC if it doesn't
	//	have the log entries required to bring a follower up to date.
	if rf.nextIndex[server] <= rf.logFirstIndexL() {
		rf.installL(server)
		return
	}

	args := rf._makeAppendArgsL(server)
	reply := &AppendEntriesReply{}

	rf.logL(VBasic, TLeader, "append to %v: %v (append)", server, args)

	rf.mu.Unlock()
	LogRaft(VExcessive, TTrace, rf.me, NA, "lock released (append)")
	ok := rf.sendAppendEntries(server, args, reply)
	LogRaft(VExcessive, TTrace, rf.me, NA, "lock acquired (append)")
	rf.mu.Lock()

	if ok {
		rf.processAppendReplyL(server, args, reply)
	} else {
		rf.logL(VVerbose, TError, "append to %v falied (append)", server)
	}
}

func (rf *Raft) _makeAppendArgsL(server int) *AppendEntriesArgs {
	nextIndex := rf.nextIndex[server]
	entries := make([]LogEntry, 0)
	if rf.lastLogL().Index >= nextIndex {
		entries = append(entries, rf.log[rf.logSliceIndexL(nextIndex):]...)
	}

	return &AppendEntriesArgs{rf.currentTerm, rf.me, nextIndex - 1,
		rf.getLogL(nextIndex - 1).Term, entries, rf.commitIndex}
}

func (rf *Raft) processAppendReplyL(server int, args *AppendEntriesArgs, reply *AppendEntriesReply) {
	rf.logL(VBasic, TAppend, "append reply from %v: %v (%v) (processAppendReplyL)",
		server, reply, args)

	if rf.currentTerm == args.Term && rf.role == Leader {
		switch reply.Success {
		case true:
			// Leaders:
			//	If successful: update nextIndex and matchIndex for follower (§5.3)
			matchIndex := args.PrevLogIndex + len(args.Entries)
			if matchIndex > rf.matchIndex[server] {
				rf.matchIndex[server] = matchIndex
				rf.nextIndex[server] = matchIndex + 1
			}
			//	If there exists an N such that N > commitIndex, a majority of matchIndex[i] ≥ N,
			//		and log[N].term == currentTerm: set commitIndex = N (§5.3, §5.4).
			if ok := rf.setCommitIndexL(); ok {
				rf.applySignalL()
			}
		case false:
			// All Servers:
			//	If RPC request or response contains term T > currentTerm: set currentTerm = T, convert to follower (§5.1)
			if reply.Term > rf.currentTerm {
				rf.becomeFollowerL(reply.Term)
				return
			}
			// Leaders:
			//	If AppendEntries fails because of log inconsistency: decrement nextIndex and retry (§5.3)
			rf._processPrevLogConflictL(server, args, reply)
		}
	}
}

func (rf *Raft) _processPrevLogConflictL(server int, args *AppendEntriesArgs, reply *AppendEntriesReply) {
	rf.logL(VBasic, TLogFail, "conflict with %v i%v/T%02d (_processPrevLogConflictL)",
		server, reply.ConflictFirstIndex, reply.ConflictTerm)

	if reply.ConflictTerm == NA {
		rf.nextIndex[server] = reply.ConflictFirstIndex
	} else {
		if reply.ConflictTerm > rf.currentTerm {
			rf.nextIndex[server] = reply.ConflictFirstIndex
		} else {
			// search for first matched index
			index := args.PrevLogIndex - 1
			minIndex := Max(reply.ConflictFirstIndex, rf.logFirstIndexL()+1)
			for index >= minIndex {
				if rf.getLogL(index).Term == reply.ConflictTerm {
					index++
					break
				}
				index--
			}
			rf.nextIndex[server] = index
		}
	}
}

func (rf *Raft) setCommitIndexL() bool {
	matchIndex := make([]int, rf.nPeers())
	copy(matchIndex, rf.matchIndex)

	sort.Ints(matchIndex)
	N := matchIndex[rf.nPeers()/2+1]
	if N > rf.commitIndex && rf.getLogL(N).Term == rf.currentTerm {
		rf.logL(VBasic, TCommit, "commit %v (setCommitIndexL)", N)

		rf.commitIndex = N
		return true
	}

	return false
}

// install snapshot

func (rf *Raft) installL(server int) {
	args := &InstallSnapshotArgs{rf.currentTerm, rf.me, rf.snapshot}
	reply := &InstallSnapshotReply{}

	rf.logL(VBasic, TSnapshot, "install to %v: %v (install)", server, args)

	rf.mu.Unlock()
	LogRaft(VExcessive, TTrace, rf.me, NA, "lock released (installL)")
	ok := rf.sendInstallSnapshot(server, args, reply)
	LogRaft(VExcessive, TTrace, rf.me, NA, "lock acquired (installL)")
	rf.mu.Lock()

	if ok {
		rf.processInstallReplyL(server, args, reply)
	} else {
		rf.logL(VVerbose, TError, "install to %v failed ()install", server)
	}
}

func (rf *Raft) processInstallReplyL(server int, args *InstallSnapshotArgs, reply *InstallSnapshotReply) {
	rf.logL(VBasic, TSnapshot, "install reply from %v: %v (%v) (processAppendReplyL)",
		server, reply, args)

	if rf.currentTerm == args.Term && rf.role == Leader {
		// All Servers:
		//	If RPC request or response contains term T > currentTerm: set currentTerm = T, convert to follower (§5.1)
		if reply.Term > rf.currentTerm {
			rf.becomeFollowerL(reply.Term)
			return
		}
		// Snapshot Additional
		if rf.matchIndex[server] < args.Snapshot.LastIncludedIndex {
			rf.matchIndex[server] = args.Snapshot.LastIncludedIndex
			rf.nextIndex[server] = rf.matchIndex[server] + 1
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
func Make(peers []*labrpc.ClientEnd, me int, persister *Persister, applyCh chan ApplyMsg) *Raft {
	// initialization
	rf := &Raft{}
	rf.initL(peers, me, persister, applyCh)

	rf.logL(VBasic, TTrace, "start peer (Make)")

	// initialize from state persisted before a crash
	rf.readPersist(persister.ReadRaftState())

	if rf.currentTerm != 0 {
		rf.resetElectionTimeoutL(false)
	}

	// start ticker goroutine to start elections
	go rf.ticker()

	go rf.apply(applyCh)

	return rf
}
