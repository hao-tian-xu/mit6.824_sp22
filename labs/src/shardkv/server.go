package shardkv

import (
	"6.824/labgob"
	"6.824/labrpc"
	"6.824/raft"
	"6.824/shardctrler"
	. "6.824/util"
	"bytes"
	"fmt"
	"log"
	"sync"
	"time"
)

// OP DATA TYPE

type Op struct {
	OpType string

	// client
	Key   string
	Value string

	// reconfiguration
	Shards  []int
	Gid     int
	KVMap   map[string]string
	LastOps map[int]LastOp

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
	if op.ClientId == op1.ClientId && op.OpId == op1.OpId && op.OpType == op1.OpType {
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

	// raft
	applyCh   chan raft.ApplyMsg
	persister *raft.Persister
	rf        *raft.Raft

	// state
	kvMap       map[string]string // kv storage
	validShards map[int]bool      // shards held by this group
	lastOps     map[int]LastOp    // clientId -> LastOp

	resultChans map[int]chan _Result // commandIndex -> chan _Result
	ctrler      *shardctrler.Clerk

	clientId int
	opId     int
}

func (kv *ShardKV) init(servers []*labrpc.ClientEnd, me int, persister *raft.Persister, maxraftstate int, gid int, ctrlers []*labrpc.ClientEnd, makeEnd func(string) *labrpc.ClientEnd) {
	kv.me = me
	kv.gid = gid

	kv.log(VBasic, TKVServer2, "start kv server")

	kv.makeEnd = makeEnd
	kv.ctrlers = ctrlers
	kv.maxraftstate = maxraftstate

	kv.applyCh = make(chan raft.ApplyMsg)
	kv.persister = persister
	kv.rf = raft.MakeName(servers, me, persister, kv.applyCh, fmt.Sprintf("GID%v", gid))

	kv.kvMap = map[string]string{}
	kv.validShards = map[int]bool{}
	kv.lastOps = map[int]LastOp{}

	kv.resultChans = map[int]chan _Result{}
	kv.ctrler = shardctrler.MakeClerk(kv.ctrlers)

	kv.clientId = -gid
	kv.opId = 0
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
		switch op.OpType {
		case opPut:
			kv.kvMap[op.Key] = op.Value
		case opAppend:
			kv.kvMap[op.Key] += op.Value
		case opGet:
			result.value = kv.kvMap[op.Key]
			if result.value == "" {
				result.err = ErrNoKey
			}
		case opAddShards:
			kv.applyAddShardsL(op)
		case opHandOffShards:
			kv.applyHandOffShardsL(op)
		}
		// Remember lastOp for each client
		kv.lastOps[op.ClientId] = LastOp{*op, result.value}

	} else {
		kv.log(VBasic, TWarn, "%v just processed... (receiveApplyMsg)", op)

		result.err = OK
		result.value = kv.lastOps[op.ClientId].GetResult
	}
	return result
}

func (kv *ShardKV) applyAddShardsL(op *Op) {
	shards, kvMap, lastOps := op.Shards, op.KVMap, op.LastOps
	// atomic: update validShards, kvMap, and lastOps
	for _, shard := range shards {
		kv.validShards[shard] = true
	}
	for k, v := range kvMap {
		if InSlice(key2shard(k), shards) {
			kv.kvMap[k] = v
		}
	}
	for clientId, lastOp := range lastOps {
		o := lastOp.Op
		if (o.OpType == opPut || o.OpType == opAppend) &&
			InSlice(key2shard(o.Key), shards) {

			kv.lastOps[clientId] = lastOp
		}
	}
}

func (kv *ShardKV) applyHandOffShardsL(op *Op) {
	shards, gid := op.Shards, op.Gid

	for _, shard := range shards {
		delete(kv.validShards, shard)
	}

	args := &HandOffShardsArgs{kv.gid, shards, kv.kvMap, kv.lastOps}
	go kv.handOff(args, gid)
}

// client end hand off

func (kv *ShardKV) handOff(args *HandOffShardsArgs, gid int) {
	for {
		servers := kv.ctrler.Query(NA).Groups[gid]
		for si := 0; si < len(servers); si++ {
			srv := kv.makeEnd(servers[si])
			reply := &HandOffShardsReply{}

			ok := srv.Call("ShardKV.HandOffShards", args, reply)

			if ok && reply.Err == OK {
				return
			}
			// ... not ok, or ErrWrongLeader
		}
		time.Sleep(HeartbeatsInterval)
	}
}

// HAND OFF SHARDS RPC HANDLER

func (kv *ShardKV) HandOffShards(args *HandOffShardsArgs, reply *HandOffShardsReply) {
	kv.lock("HandOffShards")
	defer kv.unlock("HandOffShards")

	if _, isLeader := kv.rf.GetState(); !isLeader {
		reply.Err = ErrWrongLeader
		return
	}

	if kv.noNewShardsL(args.Shards) {
		reply.Err = OK
		return
	}

	kv.addShardsL(args.Shards, args.KVMap, args.LastOps)

	if kv.noNewShardsL(args.Shards) {
		reply.Err = OK
	}
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
	var lastOps map[int]LastOp
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

func (kv *ShardKV) noNewShardsL(shards []int) bool {
	for _, shard := range shards {
		if !kv.validShards[shard] {
			return false
		}
	}
	return true
}

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

// POLL CONFIGURATION

func (kv *ShardKV) pollConfig() {
	config := kv.ctrler.Query(-1)
	for config.Num == 0 {
		time.Sleep(HeartbeatsInterval)
		config = kv.ctrler.Query(-1)
	}

	kv.log(VBasic, TConfig, "first config %v (pollConfig)", config)

	// get first valid config and add shards to 'validShards'
	shards := make([]int, 0)
	for shard, gid := range config.Shards {
		if gid == kv.gid {
			shards = append(shards, shard)
		}
	}
	kv.addShards(shards, map[string]string{}, map[int]LastOp{})

	kv._pollConfig()
}

func (kv *ShardKV) _pollConfig() {
	for {
		time.Sleep(HeartbeatsInterval)

		if _, isLeader := kv.rf.GetState(); isLeader {
			config := kv.ctrler.Query(-1)
			gidShards := kv.getGidShards(config)

			for gid, shards := range gidShards {
				kv.handOffShards(shards, gid)
			}
		}
	}
}

func (kv *ShardKV) getGidShards(config shardctrler.Config) map[int][]int {
	kv.lock("_pollConfig")
	defer kv.unlock("_pollConfig")

	gidShards := map[int][]int{}
	for shard, gid := range config.Shards {
		if kv.validShards[shard] && gid != kv.gid { // TODO: check tuple syntax
			gidShards[gid] = append(gidShards[gid], shard)
		}
	}

	return gidShards
}

// RECONFIGURATION

func (kv *ShardKV) addShards(shards []int, kvMap map[string]string, lastOps map[int]LastOp) {
	kv.lock("addShards")
	defer kv.unlock("addShards")

	kv.reconfigureL(
		&Op{OpType: opAddShards, Shards: shards, KVMap: kvMap, LastOps: lastOps})
}

func (kv *ShardKV) handOffShards(shards []int, gid int) {
	kv.lock("handOffShards")
	defer kv.unlock("handOffShards")

	kv.reconfigureL(
		&Op{OpType: opHandOffShards, Shards: shards, Gid: gid})
}

func (kv *ShardKV) reconfigureL(op *Op) {
	// MEMO: no retry

	kv.log(VBasic, TConfig, "process op %v", op)

	op.ClientId = kv.clientId
	op.OpId = kv.opId
	kv.opId++

	// return if op is the same as lastOp
	if lastOp, ok := kv.lastOps[op.ClientId]; ok && op.isSame(&lastOp.Op) {
		return
	}

	// Send op to raft by rf.Start(), return if not leader
	commandIndex, _, isLeader := kv.rf.Start(*op)
	if !isLeader {
		return
	}

	kv.waitApplyL(commandIndex, op)
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
