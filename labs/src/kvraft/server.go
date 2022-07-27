package kvraft

import (
	"6.824/labgob"
	"6.824/labrpc"
	"6.824/raft"
	"log"
	"sync"
	"sync/atomic"
	"time"
)

const (
	_NA = -1

	// log verbosity level
	vBasic LogVerbosity = iota
	vVerbose
	vExcessive

	// log topic
	tTrace LogTopic = "TRCE"
	tError LogTopic = "ERRO"

	tClient   LogTopic = "CLNT"
	tKVServer LogTopic = "KVSR"

	// timing
	_MinInterval       = 10 * time.Millisecond
	_HeartBeatInterval = 100 * time.Millisecond
)

const Debug = false

func DPrintf(format string, a ...interface{}) (n int, err error) {
	if Debug {
		log.Printf(format, a...)
	}
	return
}

// DATA TYPE

type Op struct {
	// Your definitions here.
	// Field names must start with capital letters,
	// otherwise RPC will break.
	OpType string
	Key    string
	Value  string
	// 3A Hint: You will need to uniquely identify client operations
	ClientId int
	OpId     int
}

type KVServer struct {
	mu      sync.Mutex
	me      int
	rf      *raft.Raft
	applyCh chan raft.ApplyMsg
	dead    int32 // set by Kill()

	maxraftstate int // snapshot if log grows this big

	// Your definitions here.
	kvMap     map[string]string // kv storage
	lastOpMap map[int]LastOp    // save last processed Op for eache client
	// channels for ApplyMsg transfer
	applyOpChans map[int]chan Op
}

type LastOp struct {
	op    Op
	value string
}

// RPC

func (kv *KVServer) Get(args *GetArgs, reply *OpReply) {
	// Your code here.
	op := Op{OpType: opGet, Key: args.Key, ClientId: args.ClientId, OpId: args.OpId}

	kv.processOp(opGet, &op, reply)
}

func (kv *KVServer) PutAppend(args *PutAppendArgs, reply *OpReply) {
	// Your code here.
	op := Op{OpType: args.Op, Key: args.Key, Value: args.Value, ClientId: args.ClientId, OpId: args.OpId}
	kv.processOp(args.Op, &op, reply)
}

//
// process RPC request
//
func (kv *KVServer) processOp(opType string, op *Op, reply *OpReply) {
	kv.mu.Lock()
	// If op is the same as lastOp
	if lastOp, ok := kv.lastOpMap[op.ClientId]; ok && lastOp.op == *op {
		kv.mu.Unlock()
		LogKV(vBasic, tTrace, kv.me, "processed... (%v)\n", opType)
		reply.Err, reply.Value = OK, lastOp.value
		return
	}

	// Send op to raft by rf.Start()
	commandIndex, _, isLeader, currentLeader := kv.rf.StartWithCurrentLeader(*op)
	//		if not isLeader
	if !isLeader {
		kv.mu.Unlock()
		LogKV(vVerbose, tTrace, kv.me, "notLeader... (%v)\n", opType)
		reply.Err, reply.CurrentLeader = ErrWrongLeader, currentLeader
		return
	}

	// make a channel for Op transfer, if not existing
	if _, ok := kv.applyOpChans[commandIndex]; !ok {
		kv.applyOpChans[commandIndex] = make(chan Op)
	}

	kv.mu.Unlock()

	// Wait for the op to be applied
	applied := kv.waitApply(commandIndex, op)

	kv.mu.Lock()
	value, exist := kv.kvMap[op.Key] // TODO: wrong place to check exist for both Get and Append (may let APPLY_MSG decide)
	kv.mu.Unlock()

	// Reply to client (and log)
	switch applied {
	case OK:
		switch opType {
		case opGet:
			if exist {
				LogKV(vBasic, tKVServer, kv.me, "get %v/%v! (Get)\n", op.Key, value)
				reply.Err, reply.Value = OK, value
			} else {
				LogKV(vBasic, tKVServer, kv.me, "get %v/nokey... (Get)\n", op.Key)
				reply.Err, reply.Value = ErrNoKey, ""
			}
		case opPut:
			LogKV(vBasic, tKVServer, kv.me, "put %v/%v! (Put)\n", op.Key, op.Value)
			reply.Err = OK
		case opAppend:
			if exist {
				LogKV(vBasic, tKVServer, kv.me, "append %v/%v! (Append)\n", op.Key, op.Value)
			} else {
				LogKV(vBasic, tKVServer, kv.me, "put %v/%v! (Append)\n", op.Key, op.Value)
			}
			reply.Err = OK
		}
	case ErrNotApplied:
		LogKV(vBasic, tKVServer, kv.me, "%v %v/%v notApplied... (processOp)\n", opType, op.Key, op.Value)
		reply.Err = ErrNotApplied
	case ErrTimeout:
		LogKV(vBasic, tKVServer, kv.me, "%v %v/%v timeout... (processOp)\n", opType, op.Key, op.Value)
		reply.Err = ErrTimeout
	}
}

// KILL

//
// the tester calls Kill() when a KVServer instance won't
// be needed again. for your convenience, we supply
// code to set rf.dead (without needing a lock),
// and a killed() method to test rf.dead in
// long-running loops. you can also add your own
// code to Kill(). you're not required to do anything
// about this, but it may be convenient (for example)
// to suppress debug output from a Kill()ed instance.
//
func (kv *KVServer) Kill() {
	atomic.StoreInt32(&kv.dead, 1)
	kv.rf.Kill()
	// Your code here, if desired.
}

func (kv *KVServer) killed() bool {
	z := atomic.LoadInt32(&kv.dead)
	return z == 1
}

// APPLY_MSG

//
// wait for the commandIndex to be applied in kvserver, return true if it's the same op
//
func (kv *KVServer) waitApply(commandIndex int, op *Op) Err {
	kv.mu.Lock()
	ch := kv.applyOpChans[commandIndex]
	kv.mu.Unlock()
	// Wait for the commandIndex to be commited in raft and applied in kvserver
	select {
	case appliedOp := <-ch:
		// return true if it's the same op
		if appliedOp == *op {
			LogKV(vVerbose, tTrace, kv.me, "command%v applied!", commandIndex)
			return OK
		} else {
			return ErrNotApplied
		}
	case <-time.After(_HeartBeatInterval * 5):
		return ErrTimeout
	}
}

//
// iteratively receive ApplyMsg from raft and apply it to the kvserver, notify relevant RPC (leader server)
//
func (kv *KVServer) receiveApplyMsg() {
	var applyMsg raft.ApplyMsg
	for !kv.killed() {
		// Recieve ApplyMsg from raft
		applyMsg = <-kv.applyCh
		// if ApplyMsg is a command
		if applyMsg.CommandValid {
			// map command to Op
			if op, ok := applyMsg.Command.(Op); ok {
				kv.mu.Lock()
				// command is not processed
				if lastOp, started := kv.lastOpMap[op.ClientId]; !started || op != lastOp.op {
					// Apply the command to kvserver
					value, ok := kv.kvMap[op.Key]
					switch op.OpType {
					case opPut:
						kv.kvMap[op.Key] = op.Value
					case opAppend:
						if ok {
							kv.kvMap[op.Key] += op.Value
						} else {
							kv.kvMap[op.Key] = op.Value
						}
					}
					// remember last processed op
					kv.lastOpMap[op.ClientId] = LastOp{op, value}

					if op.OpType != opGet {
						LogKV(vVerbose, tTrace, kv.me, "%v %v/%v applied! (receiveApplyMsg)\n", op.OpType, op.Key, op.Value)
					}
				} else {
					// command already processed
					LogKV(vVerbose, tTrace, kv.me, "%v %v/%v already processed... (receiveApplyMsg)\n", op.OpType, op.Key, op.Value)
				}

				// Send Op to RPC if relevant channel exists
				ch, ok := kv.applyOpChans[applyMsg.CommandIndex]
				kv.mu.Unlock()
				if ok {
					select {
					case ch <- op:
					case <-time.After(_MinInterval):
						LogKV(vVerbose, tTrace, kv.me, "chan%v blocked...\n", applyMsg.CommandIndex)
					}
				} else {
					LogKV(vVerbose, tTrace, kv.me, "chan%v doesn't exist...\n", applyMsg.CommandIndex)
				}
			} else {
				// map command to Op failed
				log.Fatalln("command not op!!")
			}
		}
	}
}

// START KVSERVER

//
// servers[] contains the ports of the set of
// servers that will cooperate via Raft to
// form the fault-tolerant key/value service.
// me is the index of the current server in servers[].
// the k/v server should store snapshots through the underlying Raft
// implementation, which should call persister.SaveStateAndSnapshot() to
// atomically save the Raft state along with the snapshot.
// the k/v server should snapshot when Raft's saved state exceeds maxraftstate bytes,
// in order to allow Raft to garbage-collect its log. if maxraftstate is -1,
// you don't need to snapshot.
// StartKVServer() must return quickly, so it should start goroutines
// for any long-running work.
//
func StartKVServer(servers []*labrpc.ClientEnd, me int, persister *raft.Persister, maxraftstate int) *KVServer {
	// call labgob.Register on structures you want
	// Go's RPC library to marshall/unmarshall.
	labgob.Register(Op{})

	kv := new(KVServer)
	kv.me = me
	kv.maxraftstate = maxraftstate

	// You may need initialization code here.
	LogKV(vVerbose, tKVServer, kv.me, "new kvServer!")

	kv.applyCh = make(chan raft.ApplyMsg)
	kv.rf = raft.Make(servers, me, persister, kv.applyCh)

	kv.kvMap = map[string]string{}
	kv.lastOpMap = map[int]LastOp{}

	kv.applyOpChans = map[int]chan Op{}

	// You may need initialization code here.
	// Start receiving ApplyMsg from raft
	go kv.receiveApplyMsg()

	return kv
}
