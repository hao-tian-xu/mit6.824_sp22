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
	"6.824/labrpc"
	"fmt"
	"log"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"
)

// GENERAL CONSTANT

const (

	// not applicable
	NA = -1

	// heartbeats
	HeartbeatsInterval = 100

	// timeout
	TimeoutMin = 1000
	TimeoutMax = 1500

	// election
	Win = iota
	Lost
)

// LOG CONFIGURATION

const (

	// verbosity setting
	VERBOSE = LogBasic

	// verbosity level
	LogBasic   LogVerbosity = 1
	LogVerbose LogVerbosity = 2

	// topic
	dClient LogTopic = "CLNT"
	dLeader LogTopic = "LEAD"
	dTerm   LogTopic = "TERM"
	dError  LogTopic = "ERRO"
	dTemp   LogTopic = "TEMP"
)

type LogVerbosity int
type LogTopic string

//
// custom log function
//
func raftLog(verbosity LogVerbosity, topic LogTopic, peerId int, format string, a ...interface{}) {
	if VERBOSE >= verbosity {
		prefix := fmt.Sprintf("%v S%02d", string(topic), peerId)
		format = prefix + format
		log.Printf(format, a...)
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
	currentTerm int         // needLock
	votedFor    int         // needLock
	log         []*LogEntry // needLock
	// Volatile state on all servers:
	commitIndex int // needLock
	lastApplied int // needLock
	// Volatile state on leaders: (Reinitialized after election)
	nextIndex  []int
	matchIndex []int
	// Additional
	nPeers   int
	timeout  bool // needLock
	isLeader bool // needLock
}

type LogEntry struct {
	Term    int
	Command []byte
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
	isleader = rf.isLeader
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
// example RequestVote RPC arguments structure.
// field names must start with capital letters!
//
type RequestVoteArgs struct {
	// Your data here (2A, 2B).
	Term         int
	CandidateId  int
	LastLogIndex int
	LastLogTerm  int
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

//
// example RequestVote RPC handler.
//
func (rf *Raft) RequestVote(args *RequestVoteArgs, reply *RequestVoteReply) {
	// Your code here (2A, 2B).
	rf.Lock()
	defer rf.Unlock()
	rf.timeout = false

	reply.Term = rf.currentTerm

	if args.Term > rf.currentTerm {
		rf.currentTerm = args.Term
		rf.votedFor = NA
		rf.isLeader = false
		raftLog(LogBasic, dTerm, rf.me, "follower in term#%v (RequestVote)\n", rf.currentTerm)
	}

	if rf.votedFor != NA || args.Term < rf.currentTerm || (args.Term == rf.currentTerm && args.LastLogIndex < len(rf.log)) {
		reply.VoteGranted = false
	} else {
		rf.votedFor = args.CandidateId
		reply.VoteGranted = true
	}
}

//
// example code to send a RequestVote RPC to a server.
// server is the index of the target server in rf.peers[].
// expects RPC arguments in args.
// fills in *reply with RPC reply, so caller should
// pass &reply.
// the types of the args and reply passed to Call() must be
// the same as the types of the arguments declared in the
// handler function (including whether they are pointers).
//
// The labrpc package simulates a lossy network, in which servers
// may be unreachable, and in which requests and replies may be lost.
// Call() sends a request and waits for a reply. If a reply arrives
// within a timeout interval, Call() returns true; otherwise
// Call() returns false. Thus Call() may not return for a while.
// A false return can be caused by a dead server, a live server that
// can't be reached, a lost request, or a lost reply.
//
// Call() is guaranteed to return (perhaps after a delay) *except* if the
// handler function on the server side does not return.  Thus there
// is no need to implement your own timeouts around Call().
//
// look at the comments in ../labrpc/labrpc.go for more details.
//
// if you're having trouble getting RPC to work, check that you've
// capitalized all field names in structs passed over RPC, and
// that the caller passes the address of the reply struct with &, not
// the struct itself.
//
func (rf *Raft) sendRequestVote(server int, args *RequestVoteArgs, reply *RequestVoteReply) bool {
	ok := rf.peers[server].Call("Raft.RequestVote", args, reply)
	return ok
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
	Term int
	//Success bool
}

//
// AppendEntries RPC handler
//
func (rf *Raft) AppendEntries(args *AppendEntriesArgs, reply *AppendEntriesReply) {
	rf.Lock()
	rf.timeout = false
	reply.Term = rf.currentTerm

	if args.Term > rf.currentTerm {
		rf.currentTerm = args.Term
		rf.votedFor = NA
		rf.isLeader = false
		rf.Unlock()
		raftLog(LogBasic, dTerm, rf.me, "follower in term#%v (AppendEntries)\n", rf.currentTerm)
	} else {
		rf.Unlock()
	}
}

//
// send a AppendEntries RPC to a server
//
func (rf *Raft) sendAppendEntries(server int, args *AppendEntriesArgs, reply *AppendEntriesReply) bool {
	ok := rf.peers[server].Call("Raft.AppendEntries", args, reply)
	return ok
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
		rf.Lock()
		rf.timeout = true
		rf.Unlock()
		randomTimeout := time.Duration(TimeoutMin + rand.Intn(TimeoutMax-TimeoutMin))
		time.Sleep(time.Millisecond * randomTimeout)
		rf.Lock()
		if rf.timeout != false {
			electionResult := rf._candidate()
			if electionResult == Win {
				rf._leader()
			}
		} else {
			rf.Unlock()
		}
	}
}

func (rf *Raft) _candidate() int {
	rf.currentTerm += 1
	raftLog(LogBasic, dTerm, rf.me, "candidate in term#%v (_candidate)\n", rf.currentTerm)
	rf.votedFor = rf.me
	rf.timeout = false

	currentTerm := rf.currentTerm
	lastLogIndex := len(rf.log)
	rf.Unlock()

	//votes := 1
	// parallel
	votes := struct {
		sync.Mutex
		n int
	}{n: 1}
	done := make(chan bool)
	for i := 0; i < rf.nPeers; i++ {
		if i != rf.me {

			// parallel
			go func(i int) {
				args := &RequestVoteArgs{currentTerm, rf.me, lastLogIndex, currentTerm - 1}
				reply := &RequestVoteReply{}
				if ok := rf.sendRequestVote(i, args, reply); ok == false {
					raftLog(LogVerbose, dError, rf.me, "candidate send RequestVote to voter#%v failed!\n", i)
					done <- true
				} else {
					if reply.Term > currentTerm {
						rf.Lock()
						rf.isLeader = false
						rf.currentTerm = reply.Term
						raftLog(LogBasic, dTemp, rf.me, "candidate receives reply from higher term, and becomes follower...\n")
						rf.Unlock()
						done <- false
					}
					if reply.VoteGranted == true {
						votes.Lock()
						votes.n += 1
						votes.Unlock()
						raftLog(LogBasic, dVote, rf.me, "candidate voted by voter#%v!\n", i)
						done <- true
					}
				}
			}(i)
		}
	}

	// parallel
	for i := 0; i < rf.nPeers-1; i++ {
		flag := <-done
		if flag == false {
			break
		}
		votes.Lock()
		if votes.n > rf.nPeers/2 {
			votes.Unlock()
			return Win
		} else {
			votes.Unlock()
		}
	}
	return Lost
}

func (rf *Raft) _leader() {
	rf.Lock()
	rf.isLeader = true
	currentTerm := rf.currentTerm
	rf.Unlock()
	raftLog(LogBasic, dLeader, rf.me, "candidate becomes leader!\n")
	for rf.killed() == false {
		rf.Lock()
		if rf.isLeader == false {
			rf.Unlock()
			raftLog(LogBasic, dTemp, rf.me, "leader becomes follower...\n")
			break
		}
		rf.Unlock()
		for i := 0; i < rf.nPeers; i++ {
			if i != rf.me {
				// parallel
				go func(i int) {
					args := &AppendEntriesArgs{currentTerm}
					reply := &AppendEntriesReply{}
					if ok := rf.sendAppendEntries(i, args, reply); ok == false {
						raftLog(LogVerbose, dError, rf.me, "leader#%v send AppendEntries to follower#%v failed!\n", i)
					} else {
						if reply.Term > currentTerm {
							rf.Lock()
							rf.isLeader = false
							rf.currentTerm = reply.Term
							raftLog(LogBasic, dTemp, rf.me, "leader receives reply from higher term, and becomes follower...\n")
							rf.Unlock()
						}
					}
				}(i)
			}
		}
		time.Sleep(time.Millisecond * HeartbeatsInterval)
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
	rf.timeout = true
	rf.isLeader = false

	// initialize from state persisted before a crash
	rf.readPersist(persister.ReadRaftState())

	// start ticker goroutine to start elections
	go rf.ticker()

	return rf
}
