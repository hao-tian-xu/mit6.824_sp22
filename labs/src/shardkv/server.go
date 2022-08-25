package shardkv

import (
	"6.824/labgob"
	"6.824/labrpc"
	"6.824/raft"
	. "6.824/util"
	"bytes"
	"fmt"
	"log"
	"sync"
	"time"
)

var kvClientId = 0

// DATA TYPE

type Op struct {
	OpType string

	// client
	Key   string
	Value string

	// identification
	ClientId int
	OpId     int
}

func (op *Op) String() string {
	content := ""
	if op.OpType == opGet {
		content = fmt.Sprint(op.Key)
	} else {
		content = fmt.Sprintf("%v:%v", op.Key, op.Value)
	}
	return fmt.Sprintf("%v%v-%v %v", op.OpType, op.ClientId, op.OpId, content)
}

func (op *Op) isSame(op1 *Op) bool {
	if op.ClientId == op1.ClientId && op.OpId == op1.OpId {
		return true
	}
	return false
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

// SHARD KV

type ShardKV struct {
	mu  sync.Mutex
	me  int
	gid int

	makeEnd      func(string) *labrpc.ClientEnd
	ctrlers      []*labrpc.ClientEnd
	maxraftstate int // snapshot if log grows this big

	kvMap map[string]string // kv storage

	lastOps     map[int]*LastOp      // clientId -> LastOp
	resultChans map[int]chan _Result // commandIndex -> chan _Result
	persister   *raft.Persister

	// as client
	me1          int
	groupLeaders map[int]int
	nextOpId     int

	// raft
	applyCh chan raft.ApplyMsg
	rf      *raft.Raft
}

func (kv *ShardKV) init(servers []*labrpc.ClientEnd, me int, persister *raft.Persister, maxraftstate int, gid int, ctrlers []*labrpc.ClientEnd, makeEnd func(string) *labrpc.ClientEnd) {
	kv.me = me
	kv.gid = gid

	kv.log(VBasic, TKVServer2, "start kv server")

	kv.makeEnd = makeEnd
	kv.ctrlers = ctrlers
	kv.maxraftstate = maxraftstate

	kv.kvMap = map[string]string{}

	kv.lastOps = map[int]*LastOp{}
	kv.resultChans = map[int]chan _Result{}
	kv.persister = persister

	kvClientId--
	kv.me1 = kvClientId
	kv.groupLeaders = map[int]int{}
	kv.nextOpId = 0

	kv.applyCh = make(chan raft.ApplyMsg)
	kv.rf = raft.MakeName(servers, me, persister, kv.applyCh, fmt.Sprintf("GID%v", gid))
}

// CLIENT RPC HANDLER

func (kv *ShardKV) Get(args *GetArgs, reply *OpReply) {
	kv.lock("Get")
	defer kv.unlock("Get")

	op := &Op{OpType: opGet, Key: args.Key,
		ClientId: args.ClientId, OpId: args.OpId}
	kv.processOpL(op, reply)
}

func (kv *ShardKV) PutAppend(args *PutAppendArgs, reply *OpReply) {
	kv.lock("Get")
	defer kv.unlock("Get")
	op := &Op{OpType: args.Op, Key: args.Key, Value: args.Value,
		ClientId: args.ClientId, OpId: args.OpId}
	kv.processOpL(op, reply)
}

//
// process RPC request
//
func (kv *ShardKV) processOpL(op *Op, reply *OpReply) {
	kv.log(VBasic, TKVServer1, "process op %v, lastOp %v (processOpL)", op, kv.lastOps[op.ClientId])

	// If op is the same as lastOp
	if lastOp, ok := kv.lastOps[op.ClientId]; ok && op.isSame(&lastOp.Op) {
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
func (kv *ShardKV) waitApplyL(commandIndex int, op *Op) _Result {
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
	if result.err == OK && !op.isSame(result.op) {
		result.err = ErrNotApplied
	}

	return result
}

// APPLIER

//
// iteratively receive ApplyMsg from raft and apply it to the kvserver, notify relevant RPC (leader server)
//
func (kv *ShardKV) receiveApplyMsg() {
	kv.lock("receiveApplyMsg-start")
	defer kv.unlock("receiveApplyMsg-end")

	for {
		kv.unlock("receiveApplyMsg")
		applyMsg := <-kv.applyCh
		kv.lock("receiveApplyMsg")

		// If ApplyMsg is a command
		if applyMsg.CommandValid {
			op := applyMsg.Command.(Op)

			result := kv.applyOpL(&op)

			if ch, ok := kv.resultChans[applyMsg.CommandIndex]; ok {
				result.op = &op

				kv.unlock("receiveApplyMsg-result")
				ch <- result
				kv.lock("receiveApplyMsg-result")
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

func (kv *ShardKV) applyOpL(op *Op) _Result {
	result := _Result{}
	// If command is not processed
	if lastOp, started := kv.lastOps[op.ClientId]; !started || !op.isSame(&lastOp.Op) {
		kv.log(VBasic, TKVServer2, "apply %v (applyOpL)", op)

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

func (kv *ShardKV) snapshotL(index int) {
	w := new(bytes.Buffer)
	e := labgob.NewEncoder(w)
	if e.Encode(kv.kvMap) != nil ||
		e.Encode(kv.lastOps) != nil {

		log.Fatalln("snapshotL() failed!")
	} else {
		kv.log(VBasic, TSnapshot, "take snapshot %v (snapshotL)", index)

		snapshot := w.Bytes()
		kv.rf.Snapshot(index, snapshot)
	}
}

func (kv *ShardKV) readSnapshotL(snapshot []byte) {
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
		kv.log(VBasic, TSnapshot, "read snapshot, lastOps %v (readSnapshotL)", lastOps)

		kv.kvMap = kvMap
		kv.lastOps = lastOps
	}
}

// HELPER

func (kv *ShardKV) log(verbose LogVerbosity, topic LogTopic, format string, a ...interface{}) {
	format = fmt.Sprintf("GID%v: ", kv.gid) + format
	peerId := kv.me + (kv.gid%3)*3

	LogKV(verbose, topic, peerId, format, a...)
}

func (kv *ShardKV) lock(method string) {
	kv.log(VExcessive, TTrace, "acquire lock (%v)", method)
	kv.mu.Lock()
}

func (kv *ShardKV) unlock(method string) {
	kv.log(VExcessive, TTrace, "release lock (%v)", method)
	kv.mu.Unlock()
}

//
// the tester calls Kill() when a ShardKV instance won't
// be needed again. you are not required to do anything
// in Kill(), but it might be convenient to (for example)
// turn off debug output from this instance.
//
func (kv *ShardKV) Kill() {
	kv.rf.Kill()
	// Your code here, if desired.
}

//
// servers[] contains the ports of the servers in this group.
//
// me is the index of the current server in servers[].
//
// the k/v server should store snapshots through the underlying Raft
// implementation, which should call persister.SaveStateAndSnapshot() to
// atomically save the Raft state along with the snapshot.
//
// the k/v server should snapshot when Raft's saved state exceeds
// maxraftstate bytes, in order to allow Raft to garbage-collect its
// log. if maxraftstate is -1, you don't need to snapshot.
//
// gid is this group's GID, for interacting with the shardctrler.
//
// pass ctrlers[] to shardctrler.MakeClerk() so you can send
// RPCs to the shardctrler.
//
// makeEnd(servername) turns a server name from a
// Config.Groups[gid][i] into a labrpc.ClientEnd on which you can
// send RPCs. You'll need this to send RPCs to other groups.
//
// look at client.go for examples of how to use ctrlers[]
// and makeEnd() to send RPCs to the group owning a specific shard.
//
// StartServer() must return quickly, so it should start goroutines
// for any long-running work.
//
func StartServer(servers []*labrpc.ClientEnd, me int, persister *raft.Persister, maxraftstate int, gid int, ctrlers []*labrpc.ClientEnd, makeEnd func(string) *labrpc.ClientEnd) *ShardKV {
	// call labgob.Register on structures you want
	// Go's RPC library to marshall/unmarshall.
	labgob.Register(Op{})

	kv := new(ShardKV)

	// Use something like this to talk to the shardctrler:
	// kv.mck = shardctrler.MakeClerk(kv.ctrlers)
	kv.init(servers, me, persister, maxraftstate, gid, ctrlers, makeEnd)

	go kv.pollConfig()

	go kv.receiveApplyMsg()

	return kv
}
