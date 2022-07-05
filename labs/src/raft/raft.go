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
	"fmt"
	"log"
	"math/rand"
	"time"

	//	"bytes"
	"sync"
	"sync/atomic"

	//	"6.824/labgob"
	"6.824/labrpc"
)

// GENERAL CONSTANT

const NA = -1

// LOG CONFIGURATION

const (
	// verbosity setting
	VERBOSE = LogNone

	// verbosity level
	LogNone    LogVerbosity = 0
	LogBasic   LogVerbosity = 1
	LogVerbose LogVerbosity = 2

	// topic
	dLeader LogTopic = "LEAD"
	dTerm   LogTopic = "TERM"
	dError  LogTopic = "ERRO"
	dTemp   LogTopic = "TEMP"
	dVote   LogTopic = "VOTE"
)

type (
	LogVerbosity int
	LogTopic     string
)

//
// custom log function
//
func raftLog(verbosity LogVerbosity, topic LogTopic, peerId int, format string, a ...interface{}) {
	if VERBOSE >= verbosity {
		go func() {
			prefix := fmt.Sprintf("%v S%02d ", string(topic), peerId)
			format = prefix + format
			log.Printf(format, a...)
		}()
	}
}

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
	lastLogIndex int
	lastLogTerm  int
}

type LogEntry struct {
	Term    int
	Command []byte
}

type PeerRole int

const (
	Leader PeerRole = iota
	Candidate
	Follower
)

// TIMING

const (
	// heartbeats
	HeartbeatsInterval = 100 * time.Millisecond

	// timeout
	TimeoutMin = 1000
	TimeoutMax = 1500
)

func randomTimeout() time.Duration {
	return time.Millisecond * time.Duration(TimeoutMin+rand.Intn(TimeoutMax-TimeoutMin))
}

// GET STATE

// return currentTerm and whether this server
// believes it is the leader.
func (rf *Raft) GetState() (int, bool) {

	var term int
	var isleader bool
	// Your code here (2A).
	rf.Lock()
	defer rf.Unlock()
	term = rf.currentTerm
	isleader = rf.role == Leader

	return term, isleader
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
		rf.currentTerm = args.Term
		rf.votedFor = NA
		rf.role = Follower

		raftLog(LogBasic, dTerm, rf.me, "follower in term#%v (RequestVote)\n", rf.currentTerm)
	}

	reply.Term = rf.currentTerm

	// Reply false if term < currentTerm
	// If votedFor is null or candidateId, and candidate’s log is at least as up-to-date as receiver’s log, grant vote
	// TODO: read the paper to verify
	if args.Term >= rf.currentTerm &&
		(rf.votedFor == NA || rf.votedFor == args.CandidateId) &&
		(args.LastLogTerm >= rf.lastLogTerm && args.LastLogIndex >= rf.lastLogIndex) {

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

	if !ok {
		raftLog(LogVerbose, dError, rf.me, "candidate send RequestVote to voter#%v failed!\n", server)
	} else {
		if reply.VoteGranted == true {
			voteCounter.Lock()
			voteCounter.n += 1
			voteCounter.Unlock()

			raftLog(LogBasic, dVote, rf.me, "candidate voted by voter#%v!\n", server)
		} else {
			// All Servers: If RPC request or response contains term T > currentTerm: set currentTerm = T, convert to follower
			if reply.Term > term {
				rf.Lock()
				rf.currentTerm = reply.Term
				rf.votedFor = NA
				rf.role = Follower
				rf.Unlock()

				raftLog(LogBasic, dTemp, rf.me, "candidate receives reply from higher term, and becomes follower...\n")
			}
		}

	}
}

// APPEND ENTRIES RPC

//
// AppendEntries RPC arguments structure
//
type AppendEntriesArgs struct {
	Term int
	//LeaderId     int
	//PrevLogIndex int
	//PrevLogTerm  int
	//Entries      []*LogEntry
	//LeaderCommit int
}

//
// AppendEntries RPC reply structure
//
type AppendEntriesReply struct {
	Term    int
	Success bool
}

//
// AppendEntries RPC handler
//
func (rf *Raft) AppendEntries(args *AppendEntriesArgs, reply *AppendEntriesReply) {
	rf.Lock()
	defer rf.Unlock()

	reply.Term = rf.currentTerm

	// If AppendEntries RPC received from new leader: convert to follower
	if rf.role == Candidate && rf.currentTerm <= args.Term {
		rf.role = Follower
	}

	// All Servers: If RPC request or response contains term T > currentTerm: set currentTerm = T, convert to follower
	if args.Term > rf.currentTerm {
		rf.currentTerm = args.Term
		rf.votedFor = NA
		rf.role = Follower

		raftLog(LogBasic, dTerm, rf.me, "follower in term#%v (AppendEntries)\n", rf.currentTerm)
	}

	// Reply false if term < currentTerm
	if args.Term < rf.currentTerm {
		reply.Success = false
	} else {
		reply.Success = true

		// Followers: reset election timeout if receiving AppendEntries RPC from current leader
		rf.timeoutStart = time.Now()
	}

}

//
// send a AppendEntries RPC to a server and process the reply
//
func (rf *Raft) sendAppendEntries(server int, currentTerm int) {
	args := &AppendEntriesArgs{currentTerm}
	reply := &AppendEntriesReply{}

	ok := rf.peers[server].Call("Raft.AppendEntries", args, reply)

	if !ok {
		raftLog(LogVerbose, dError, rf.me, "leader send AppendEntries to follower#%v failed!\n", server)
	} else {
		// All Servers: If RPC request or response contains term T > currentTerm: set currentTerm = T, convert to follower
		if !reply.Success {
			if reply.Term > currentTerm {
				rf.Lock()
				rf.currentTerm = args.Term
				rf.votedFor = NA
				rf.role = Follower
				rf.Unlock()

				raftLog(LogBasic, dTemp, rf.me, "leader receives reply from higher term, and becomes follower...\n")
			}
		}
	}
}

// 2A UNUSED

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
	// On conversion to candidate, start election
	rf.Lock()

	rf.role = Candidate
	rf.currentTerm += 1
	raftLog(LogBasic, dTerm, rf.me, "candidate in term#%v (candidate)\n", rf.currentTerm)
	rf.votedFor = rf.me
	rf.timeoutStart = time.Now()

	// Prepare for RequestVote
	term := rf.currentTerm
	lastLogIndex := rf.lastLogIndex
	lastLogTerm := term - 1 // TODO: maybe wrong

	rf.Unlock()

	// Send RequestVote RPCs to all other servers
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
		// condition 1. If AppendEntries RPC received from new leader: convert to follower
		// other conditions
		if role == Follower {
			break
		}

		// If election timeout elapses: start new election
		if timeout > randomTimeout {
			go rf.candidate()
			break
		}

		// If votes received from majority of servers: become leader
		voteCounter.Lock()
		nVote := voteCounter.n
		voteCounter.Unlock()
		if nVote > rf.nPeers/2 {
			go rf.leader()
			break
		}

		// Intervals for candidate to recheck
		time.Sleep(HeartbeatsInterval / 10)
	}
}

func (rf *Raft) leader() {
	rf.Lock()
	rf.role = Leader
	currentTerm := rf.currentTerm
	rf.Unlock()
	raftLog(LogBasic, dLeader, rf.me, "candidate becomes leader!\n")

	for rf.killed() == false {
		rf.Lock()
		role := rf.role
		rf.Unlock()

		// Not leader
		if role != Leader {
			raftLog(LogBasic, dTemp, rf.me, "leader becomes follower...\n")
			break
		}

		for i := 0; i < rf.nPeers; i++ {
			if i != rf.me {
				go rf.sendAppendEntries(i, currentTerm)
			}
		}

		time.Sleep(HeartbeatsInterval)
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
	rf.log = make([]*LogEntry, 0) // TODO: first index is 1?

	rf.commitIndex = 0
	rf.lastApplied = 0

	rf.nextIndex = make([]int, nPeers)
	rf.matchIndex = make([]int, nPeers)

	rf.nPeers = nPeers
	rf.timeoutStart = time.Now()
	rf.role = Follower
	rf.lastLogIndex = 0
	rf.lastLogTerm = 0

	// initialize from state persisted before a crash
	rf.readPersist(persister.ReadRaftState())

	// start ticker goroutine to start elections
	go rf.ticker()

	return rf
}
