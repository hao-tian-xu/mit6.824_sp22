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

func (l LogEntry) String() string {
	return fmt.Sprintf("ind%v/term%v", l.Index, l.Term)
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
}

// INTERFACE

//
// return currentTerm and whether this server
// believes it is the leader.
//
func (rf *Raft) GetState() (int, bool) {
	// Your code here (2A).
	rf.mu.Lock()
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
	rf.mu.Lock()
	defer rf.mu.Unlock()

	isLeader := rf.role == Leader
	if !isLeader {
		return NA, NA, isLeader
	}

	index := rf.lastLogIndexLck() + 1
	term := rf.currentTerm

	logEntry := LogEntry{command, index, term}

	LogRaft(VBasic, TClient, rf.me, "append log: %v", logEntry)

	// Leaders:
	//	If command received from client: append entry to local log
	rf.log = append(rf.log, logEntry)
	rf.persistLck()
	rf.appendEntriesLck()

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

func (rf *Raft) init(peers []*labrpc.ClientEnd, me int, persister *Persister, ch chan ApplyMsg) {
	rf.peers = peers
	rf.persister = persister
	rf.me = me

	rf.currentTerm = 0
	rf.votedFor = NA
	rf.log = make([]LogEntry, 1)
	rf.log[0] = LogEntry{Index: 0, Term: 0}

	rf.commitIndex = 0
	rf.lastApplied = 0

	rf.nextIndex = make([]int, rf.nPeers())
	rf.matchIndex = make([]int, rf.nPeers())

	rf.role = Follower
	rf.resetElectionTimeoutLck(true)
	rf.applyCond = *sync.NewCond(&rf.mu)
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
		rf.persistLck()
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

// AppendEntries RPC Handler

//
// Invoked by leader to replicate log entries (§5.3); also used as heartbeat (§5.2).
//
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

	LogRaft(VBasic, TAppend, rf.me, "append (%v) from %v: lastLog %v (AppendEntries)",
		args, args.LeaderId, rf.log[rf.lastLogIndexLck()])

	reply.Term = rf.currentTerm
	reply.Success = true

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

	// Receiver implementation:
	//	2. Reply false if log doesn’t contain an entry at prevLogIndex whose term matches prevLogTerm (§5.3)
	if rf.prevLogConflictLck(args, reply) {
		reply.Success = false
		return
	}
	//	3. If an existing entry conflicts with a new one (same index but different terms),
	//		delete the existing entry and all that follow it (§5.3)
	//	4. Append any new entries not already in the log
	for i, entry := range args.Entries {
		index := args.PrevLogIndex + 1 + i
		if rf.lastLogIndexLck() < index || entry.Term != rf.log[index].Term {
			rf.log = append(rf.log[:index], args.Entries[i:]...)
			rf.persistLck()
			break
		}
	}
	//	5. If leaderCommit > commitIndex, set commitIndex = min(leaderCommit, index of last new entry)
	if args.LeaderCommit > rf.commitIndex {
		rf.commitIndex = Min(args.LeaderCommit, rf.lastLogIndexLck())

		LogRaft(VBasic, TCommit, rf.me, "commit %v (AppendEntries)", rf.commitIndex)

		// All Servers:
		//	If commitIndex > lastApplied: increment lastApplied, apply log[lastApplied] to state machine (§5.3)
		rf.applySignalLck()
	}
}

func (rf *Raft) prevLogConflictLck(args *AppendEntriesArgs, reply *AppendEntriesReply) bool {
	if rf.lastLogIndexLck() < args.PrevLogIndex {
		// log doesn’t contain an entry at prevLogIndex
		reply.ConflictTerm = NA
		reply.ConflictFirstIndex = rf.lastLogIndexLck() + 1
		return true
	} else if rf.log[args.PrevLogIndex].Term != args.PrevLogTerm {
		// term of log's entry at prevLogIndex doesn't matches prevLogTerm
		reply.ConflictTerm = rf.log[args.PrevLogIndex].Term
		index := args.PrevLogIndex - 1
		for index > 0 {
			if rf.log[index].Term != reply.ConflictTerm {
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
	LogRaft(VBasic, TLeader, rf.me, "appendEntries to %v: %v (sendAppendEntries)", server, args)

	return rf.peers[server].Call(rpcAppendEntries, args, reply)
}

// PERSISTOR

//
// save Raft's persistent state to stable storage,
// where it can later be retrieved after a crash and restart.
// see paper's Figure 2 for a description of what should be persistent.
//
func (rf *Raft) persistLck() {
	// rf.persister.SaveRaftState(data)
	w := new(bytes.Buffer)
	e := labgob.NewEncoder(w)
	if e.Encode(rf.currentTerm) != nil ||
		e.Encode(rf.votedFor) != nil ||
		e.Encode(rf.log) != nil {

		log.Fatalln("persistLck() failed!")
	} else {
		data := w.Bytes()
		rf.persister.SaveRaftState(data)
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
	}
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
	rf.mu.Lock()
	defer rf.mu.Unlock()

	LogRaft(VVerbose, TTick, rf.me, "role: %v (tick)", rf.role)

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
	rf.persistLck()

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
	rf.persistLck()
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

	if ok {
		rf.processRequestVoteReply(server, args, reply, votes)
	} else {
		LogRaft(VVerbose, TError, rf.me, "sendRequestVote to %v falied (appendEntries)", server)
	}
}

func (rf *Raft) processRequestVoteReply(server int, args *RequestVoteArgs, reply *RequestVoteReply, votes *int) {
	rf.mu.Lock()
	defer rf.mu.Unlock()

	LogRaft(VBasic, TCandidate, rf.me, "vote reply from %v in term %v: %v (processRequestVoteReply)", server, rf.currentTerm, reply)

	if rf.currentTerm == args.Term && rf.role == Candidate {
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

	if ok {
		rf.processAppendEntriesReply(server, args, reply)
	} else {
		LogRaft(VVerbose, TError, rf.me, "sendAppendEntries to %v falied (appendEntries)", server)
	}
}

func (rf *Raft) makeAppendEntriesArgs(server int) *AppendEntriesArgs {
	rf.mu.Lock()
	defer rf.mu.Unlock()

	nextIndex := rf.nextIndex[server]
	if nextIndex <= rf.log[0].Index {
		nextIndex = rf.log[0].Index + 1
	}
	entries := make([]LogEntry, 0)
	if rf.lastLogIndexLck() >= nextIndex {
		entries = append(entries, rf.log[nextIndex:]...)
	}

	return &AppendEntriesArgs{rf.currentTerm, rf.me, nextIndex - 1,
		rf.log[nextIndex-1].Term, entries, rf.commitIndex}
}

func (rf *Raft) processAppendEntriesReply(server int, args *AppendEntriesArgs, reply *AppendEntriesReply) {
	rf.mu.Lock()
	defer rf.mu.Unlock()

	LogRaft(VBasic, TLeader, rf.me, "append (%v) reply from %v in term %v: %v (processAppendEntriesReply)",
		args, server, rf.currentTerm, reply)

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
			if ok := rf.setCommitIndexLck(); ok {
				rf.applySignalLck()
			}
		case false:
			// All Servers:
			//	If RPC request or response contains term T > currentTerm: set currentTerm = T, convert to follower (§5.1)
			if rf.currentTerm < reply.Term {
				rf.becomeFollowerLck(args.Term)
				return
			}
			// Leaders:
			//	If AppendEntries fails because of log inconsistency: decrement nextIndex and retry (§5.3)
			rf.processPrevLogConflictLck(server, args, reply)
		}
	}
}

func (rf *Raft) processPrevLogConflictLck(server int, args *AppendEntriesArgs, reply *AppendEntriesReply) {
	LogRaft(VBasic, TLogFail, rf.me, "conflict with %v ind%v/term%v, leader ind%v/term%v (processPrevLogConflictLck)",
		server, reply.ConflictFirstIndex, reply.ConflictTerm, reply.ConflictFirstIndex, rf.log[reply.ConflictFirstIndex].Term)

	if reply.ConflictTerm == NA {
		rf.nextIndex[server] = reply.ConflictFirstIndex
	} else {
		if reply.ConflictTerm > rf.currentTerm {
			rf.nextIndex[server] = reply.ConflictFirstIndex
		} else {
			index := args.PrevLogIndex - 1
			for index >= reply.ConflictFirstIndex {
				if rf.log[index].Term == reply.ConflictTerm {
					index++
					break
				}
				index--
			}
			rf.nextIndex[server] = index
		}
	}
}

func (rf *Raft) setCommitIndexLck() bool {
	matchIndex := make([]int, rf.nPeers())
	copy(matchIndex, rf.matchIndex)

	sort.Ints(matchIndex)
	N := matchIndex[rf.nPeers()/2+1]
	if N > rf.commitIndex && rf.log[N].Term == rf.currentTerm {
		LogRaft(VBasic, TCommit, rf.me, "commit %v (setCommitIndexLck)", N)

		rf.commitIndex = N
		return true
	}

	return false
}

// APPLY

func (rf *Raft) apply(applyCh chan ApplyMsg) {
	rf.mu.Lock()
	defer rf.mu.Unlock()

	for !rf.killed() {
		// All Servers:
		//	If commitIndex > lastApplied: increment lastApplied, apply log[lastApplied] to state machine (§5.3)
		if rf.commitIndex > rf.lastApplied {
			rf.lastApplied++
			applyMsg := ApplyMsg{
				CommandValid: true,
				Command:      rf.log[rf.lastApplied].Command,
				CommandIndex: rf.lastApplied,
			}

			rf.mu.Unlock()
			applyCh <- applyMsg
			rf.mu.Lock()

			LogRaft(VBasic, TApply, rf.me, "apply %v (apply)", rf.lastApplied)
		}

		if rf.commitIndex == rf.lastApplied {
			rf.applyCond.Wait()
		}
	}
}

func (rf *Raft) applySignalLck() {
	rf.applyCond.Signal()
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

	go rf.apply(applyCh)

	return rf
}
