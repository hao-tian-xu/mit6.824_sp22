package kvraft

import (
	"6.824/labgob"
	"6.824/labrpc"
	"6.824/raft"
	"bytes"
	"fmt"
	"log"
	"sync"
	"sync/atomic"
	"time"
)
import . "6.824/util"

// DATA TYPE

type Op struct {
	OpType string
	Key    string
	Value  string
	// 3A Hint: You will need to uniquely identify client operations
	ClientId int
	OpId     int
}

func (op *Op) String() string {
	if op.OpType == opGet {
		return fmt.Sprintf("%v%v-%v %v", op.OpType, op.ClientId, op.OpId, op.Key)
	} else {
		return fmt.Sprintf("%v%v-%v %v:%v", op.OpType, op.ClientId, op.OpId, op.Key, op.Value)
	}
}

type LastOp struct {
	Op        Op
	GetResult string
}

func (l *LastOp) String() string {
	return fmt.Sprintf("%v-%v", l.Op.ClientId, l.Op.OpId)
}

//
// transfer op result among methods
//
type _Result struct {
	err   Err
	value string
	op    *Op
}

func (r _Result) String() string {
	return fmt.Sprintf("%v %v", r.err, r.op)
}

// KV SERVER

type KVServer struct {
	mu      sync.Mutex
	me      int
	rf      *raft.Raft
	applyCh chan raft.ApplyMsg
	dead    int32 // set by Kill()

	maxraftstate int // snapshot if log grows this big

	kvMap map[string]string // kv storage

	lastOps     map[int]*LastOp      // clientId -> LastOp
	resultChans map[int]chan _Result // commandIndex -> chan _Result
	persister   *raft.Persister
}

// RPC HANDLER

func (kv *KVServer) Get(args *GetArgs, reply *OpReply) {
	kv.lock("Get")
	defer kv.unlock("Get")

	op := &Op{OpType: opGet, Key: args.Key,
		ClientId: args.ClientId, OpId: args.OpId}
	kv.processOpL(op, reply)
}

func (kv *KVServer) PutAppend(args *PutAppendArgs, reply *OpReply) {
	kv.lock("Get")
	defer kv.unlock("Get")
	op := &Op{OpType: args.Op, Key: args.Key, Value: args.Value,
		ClientId: args.ClientId, OpId: args.OpId}
	kv.processOpL(op, reply)
}

//
// process RPC request
//
func (kv *KVServer) processOpL(op *Op, reply *OpReply) {
	kv.log(VBasic, TKVServer1, "process op %v, lastOp %v (processOpL)", op, kv.lastOps[op.ClientId])

	// If op is the same as lastOp
	if lastOp, ok := kv.lastOps[op.ClientId]; ok && lastOp.Op == *op {
		reply.Err, reply.Value = OK, lastOp.GetResult
		return
	}

	// Send op to raft by rf.Start()
	commandIndex, _, isLeader := kv.rf.Start(*op)
	//	if not leader
	if !isLeader {
		reply.Err = ErrWrongLeader
		return
	}

	// Wait for the op to be applied
	result := kv.waitApplyL(commandIndex, op)

	reply.Err = result.err
	reply.Value = result.value
}

//
// wait for the commandIndex to be applied in kvserver, return true if it's the same op
//
func (kv *KVServer) waitApplyL(commandIndex int, op *Op) _Result {
	// Make a channel for _Result transfer, if not existing
	if _, ok := kv.resultChans[commandIndex]; !ok {
		kv.resultChans[commandIndex] = make(chan _Result)
	}

	ch := kv.resultChans[commandIndex]
	result := _Result{}

	// Wait for the commandIndex to be commited in raft and applied in kvserver
	kv.unlock("waitApply")
	select {
	case result = <-ch:
	case <-time.After(MaxTimeout * HeartbeatsInterval):
		result.err = ErrTimeout
	}
	kv.lock("waitApply")

	delete(kv.resultChans, commandIndex)

	kv.log(VBasic, TKVServer1, "command %v apply result: %v (waitApplyL)", commandIndex, result)

	// Process result
	if result.err == OK && *op != *result.op {
		result.err = ErrNotApplied
	}

	return result
}

// APPLIER

//
// iteratively receive ApplyMsg from raft and apply it to the kvserver, notify relevant RPC (leader server)
//
func (kv *KVServer) receiveApplyMsg() {
	kv.lock("receiveApplyMsg")
	defer kv.unlock("receiveApplyMsg")

	for !kv.killed() {
		kv.unlock("receiveApplyMsg")
		applyMsg := <-kv.applyCh
		kv.lock("receiveApplyMsg")

		// If ApplyMsg is a command
		if applyMsg.CommandValid {
			op := applyMsg.Command.(Op)

			result := kv.applyOpL(&op)

			if ch, ok := kv.resultChans[applyMsg.CommandIndex]; ok {
				result.op = &op

				kv.unlock("receiveApplyMsg")
				ch <- result
				kv.lock("receiveApplyMsg")
			}

			// Take a snapshot if Raft state size is approaching maxraftstate
			if kv.maxraftstate != NA && kv.persister.RaftStateSize() >= kv.maxraftstate {
				kv.snapshotL(applyMsg.CommandIndex)
			}
		}

		// If ApplyMsg is a snapshot
		if applyMsg.SnapshotValid {
			kv.readSnapshotL(applyMsg.Snapshot)
		}
	}
}

func (kv *KVServer) applyOpL(op *Op) _Result {
	result := _Result{}
	// If command is not processed
	if lastOp, started := kv.lastOps[op.ClientId]; !started || *op != lastOp.Op {
		kv.log(VBasic, TKVServer2, "%v applied! (receiveApplyMsg)", op)

		result.err = OK

		// Apply the command to kv server
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
		case opGet:
			if ok {
				result.value = value
			} else {
				result.err = ErrNoKey
				result.value = ""
			}
		}
		// Remember lastOp for each client
		kv.lastOps[op.ClientId] = &LastOp{*op, value}

	} else {
		kv.log(VBasic, TWarn, "%v just processed... (receiveApplyMsg)", op)

		result.err = ErrDuplicate
	}
	return result
}

// snapshot

func (kv *KVServer) snapshotL(index int) {
	w := new(bytes.Buffer)
	e := labgob.NewEncoder(w)
	if e.Encode(kv.kvMap) != nil ||
		e.Encode(kv.lastOps) != nil {

		log.Fatalln("snapshotL() failed!")
	} else {
		kv.log(VBasic, TSnapshot, "take snapshot %v", index)

		snapshot := w.Bytes()
		kv.rf.Snapshot(index, snapshot)
	}
}

func (kv *KVServer) readSnapshotL(snapshot []byte) {
	if snapshot == nil || len(snapshot) < 1 {
		return
	}

	r := bytes.NewBuffer(snapshot)
	d := labgob.NewDecoder(r)
	var kvMap map[string]string
	var lastOps map[int]*LastOp
	if d.Decode(&kvMap) != nil ||
		d.Decode(&lastOps) != nil {

		log.Fatalln("readSnapshotL() failed!")
	} else {
		kv.log(VBasic, TSnapshot, "read snapshot, lastOps %v", lastOps)

		kv.kvMap = kvMap
		kv.lastOps = lastOps
	}
}

// HELPER

func (kv *KVServer) log(verbose LogVerbosity, topic LogTopic, format string, a ...interface{}) {
	LogKV(verbose, topic, kv.me, format, a...)
}

func (kv *KVServer) lock(method string) {
	kv.log(VExcessive, TTrace, "acquire lock (%v)", method)
	kv.mu.Lock()
}

func (kv *KVServer) unlock(method string) {
	kv.log(VExcessive, TTrace, "release lock (%v)", method)
	kv.mu.Unlock()
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

// START

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

	kv.log(VBasic, TKVServer2, "start kv server")

	kv.applyCh = make(chan raft.ApplyMsg)
	kv.rf = raft.Make(servers, me, persister, kv.applyCh)

	kv.kvMap = map[string]string{}

	kv.lastOps = map[int]*LastOp{}
	kv.resultChans = map[int]chan _Result{}
	kv.persister = persister

	// Use snapshot to recover
	kv.readSnapshotL(kv.persister.ReadSnapshot())

	// You may need initialization code here.
	// Start receiving ApplyMsg from raft
	go kv.receiveApplyMsg()

	return kv
}
