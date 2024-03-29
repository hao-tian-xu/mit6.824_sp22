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
	"reflect"
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
	ConfigNum int
	Shards    []int
	Gid       int
	KVMap     map[string]string
	LastOps   map[int]LastOp
	Handoff   Handoff

	// identification
	ClientId int
	OpId     int
}

func (op *Op) String() string {
	content := ""
	if op.OpType == opAddShards {
		content = fmt.Sprintf("%v: %v", op.ConfigNum, op.Shards)
	} else if op.OpType == opRemoveShards || op.OpType == opHandOffDone {
		content = fmt.Sprintf("%v: %v to %v", op.ConfigNum, op.Shards, op.Gid)
	} else if op.OpType == opGet {
		content = fmt.Sprint(op.Key)
	} else {
		content = fmt.Sprintf("%v: %v", op.Key, op.Value)
	}
	return fmt.Sprintf("%v-%v %v %v", op.ClientId, op.OpId, op.OpType, content)
}

func (op *Op) isSame(clientId int, opId int) bool {
	if op.ClientId == clientId && op.OpId == opId {
		return true
	}
	return false
}

type LastOp struct {
	ClientId int
	OpId     int

	GetResult string
}

func (l LastOp) String() string {
	return fmt.Sprintf("%v-%v", l.ClientId, l.OpId)
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
	return fmt.Sprintf("%v %v (%v)", r.err, r.value, r.op)
}

type Handoff struct {
	Gid       int
	ConfigNum int
	Shards    *[]int
}

// SHARD KV

type ShardKV struct {
	mu sync.Mutex

	me   int
	gid  int
	name string

	makeEnd      func(string) *labrpc.ClientEnd
	ctrlers      []*labrpc.ClientEnd
	maxraftstate int // snapshot if log grows this big

	// raft
	applyCh   chan raft.ApplyMsg
	persister *raft.Persister
	rf        *raft.Raft

	// storage state
	kvMap       map[string]string // kv storage
	validShards map[int]bool      // shards held by this group
	lastOps     map[int]LastOp    // clientId -> LastOp
	// handOffL state
	receivedHandoffs map[int][][]int  // configNum -> []shards
	pendingHandoffs  map[Handoff]bool // configNum -> (gid -> shards)

	resultChans map[int]chan _Result // commandIndex -> chan _Result
	ctrler      *shardctrler.Clerk

	clientId int
	opId     int
}

func (kv *ShardKV) init(servers []*labrpc.ClientEnd, me int, persister *raft.Persister, maxraftstate int, gid int, ctrlers []*labrpc.ClientEnd, makeEnd func(string) *labrpc.ClientEnd) {
	kv.me = me
	kv.gid = gid
	kv.name = fmt.Sprintf("GID%v-%v", gid, kv.me)

	kv.log(VVerbose, TKVServer2, "start kv server (init)")

	kv.makeEnd = makeEnd
	kv.ctrlers = ctrlers
	kv.maxraftstate = maxraftstate

	kv.applyCh = make(chan raft.ApplyMsg)
	kv.persister = persister
	kv.rf = raft.MakeName(servers, me, persister, kv.applyCh, kv.name)

	kv.kvMap = map[string]string{}
	kv.validShards = map[int]bool{}
	kv.lastOps = map[int]LastOp{}
	kv.receivedHandoffs = map[int][][]int{}
	kv.pendingHandoffs = map[Handoff]bool{}

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
	if lastOp, ok := kv.lastOps[op.ClientId]; ok && op.isSame(lastOp.ClientId, lastOp.OpId) {
		reply.Err, reply.Value = OK, lastOp.GetResult
		return
	}

	//// If shard is not in the group
	//if !kv.validShards[key2shard(op.Key)] {
	//	reply.Err = ErrWrongGroup
	//	return
	//}

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

	kv.log(VBasic, topic(op), "command %v apply result: %v (waitApplyL)", commandIndex, result)

	// Process result
	if result.err == OK && !op.isSame(result.op.ClientId, result.op.OpId) {
		result.err = ErrNotApplied
	}

	return result
}

// HANDOFFSHARDS RPC HANDLER

func (kv *ShardKV) HandOffShards(args *HandOffShardsArgs, reply *HandOffShardsReply) { // MEMO: multi receptions
	kv.lock("HandOffShards")
	defer kv.unlock("HandOffShards")

	kv.log(VVerbose, TConfig2, "handOffShards from %v-%v: %v (HandOffShards)", args.Gid, args.Sid, args.Shards)

	if _, isLeader := kv.rf.GetState(); !isLeader {
		reply.Err = ErrWrongLeader
		return
	}

	if kv.notNewShardsL(args.ConfigNum, args.Shards) {
		reply.Err = OK
		return
	}

	kv.addShardsL(args.ConfigNum, args.Shards, args.KVMap, args.LastOps)

	if kv.notNewShardsL(args.ConfigNum, args.Shards) {
		reply.Err = OK
	}
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

			kv.log(VBasic, TSnapshot, "read snapshot %v (receiveApplyMsg)", applyMsg.CommandIndex)

			kv.readSnapshotL(applyMsg.Snapshot)
		}
	}
}

func (kv *ShardKV) applyOpL(op *Op) _Result {
	result := _Result{}
	// If command is not processed
	if lastOp, started := kv.lastOps[op.ClientId]; !started || !op.isSame(lastOp.ClientId, lastOp.OpId) {

		kv.log(VVerbose, topic(op), "apply %v (applyOpL)", op)

		// wrong group
		if op.OpType != opAddShards && op.OpType != opRemoveShards && !kv.validShards[key2shard(op.Key)] {
			result.err = ErrWrongGroup
			return result
		}

		result.err = OK

		// Apply the command to kv server
		switch op.OpType {
		case opPut:
			kv.kvMap[op.Key] = op.Value
			result.value = kv.kvMap[op.Key]
		case opAppend:
			kv.kvMap[op.Key] += op.Value
			result.value = kv.kvMap[op.Key]
		case opGet:
			result.value = kv.kvMap[op.Key]
			if result.value == "" {
				result.err = ErrNoKey
			}
		case opAddShards:
			kv.applyAddShardsL(op)
		case opRemoveShards:
			kv.applyRemoveShardsL(op)
		case opHandOffDone:
			delete(kv.pendingHandoffs, op.Handoff)
		}
		// Remember lastOp for each client
		kv.lastOps[op.ClientId] = LastOp{op.ClientId, op.OpId, result.value}

	} else {
		kv.log(VBasic, TWarn, "%v just processed... (receiveApplyMsg)", op)

		result.err = OK
		result.value = kv.lastOps[op.ClientId].GetResult
	}
	return result
}

func (kv *ShardKV) applyAddShardsL(op *Op) {
	if kv.notNewShardsL(op.ConfigNum, op.Shards) {
		// already added
		return
	}
	kv.receivedHandoffs[op.ConfigNum] = append(kv.receivedHandoffs[op.ConfigNum], op.Shards)

	// atomic: update validShards, kvMap, and lastOps
	for _, shard := range op.Shards {
		kv.validShards[shard] = true
	}
	for k, v := range op.KVMap {
		if InSlice(key2shard(k), op.Shards) {
			kv.kvMap[k] = v
		}
	}
	for clientId, lastOp := range op.LastOps { // MEMO: not 1000% sure
		if lastOp.OpId > kv.lastOps[clientId].OpId {
			kv.lastOps[clientId] = lastOp
		}
		//o := lastOp.Op
		//if (o.OpType == opPut || o.OpType == opAppend) &&
		//	InSlice(key2shard(o.Key), op.Shards) {
		//
		//	kv.lastOps[clientId] = lastOp
		//}
	}

	kv.unlock("applyAddShardsL")
	if _, isLeader := kv.rf.GetState(); isLeader {
		kv.log(VTemp, TTrace, "received kvMap: %v (applyAddShardsL)", op.KVMap)
		kv.log(VTemp, TTrace, "current kvMap: %v (applyAddShardsL)", kv.kvMap)
	}
	kv.lock("applyAddShardsL")
}

func (kv *ShardKV) applyRemoveShardsL(op *Op) {
	if kv.removed(op) {
		return
	}

	for _, shard := range op.Shards {
		delete(kv.validShards, shard)
	}

	kv.pendingHandoffs[Handoff{op.Gid, op.ConfigNum, &op.Shards}] = true
}

func (kv *ShardKV) removed(op *Op) bool {
	for _, shard := range op.Shards {
		if !kv.validShards[shard] {
			return true
		}
	}
	return false
}

//func (kv *ShardKV) pendHandoffsL(op *Op) {
//	if _, ok := kv.pendingHandoffs[op.ConfigNum]; !ok {
//		kv.pendingHandoffs[op.ConfigNum] = map[int][][]int{}
//	}
//
//	for _, shards := range kv.pendingHandoffs[op.ConfigNum][op.Gid] {
//		if reflect.DeepEqual(shards, op.Shards) {
//			return
//		}
//	}
//
//	kv.pendingHandoffs[op.ConfigNum][op.Gid] = append(kv.pendingHandoffs[op.ConfigNum][op.Gid], op.Shards)
//}

// snapshot

func (kv *ShardKV) snapshotL(index int) {
	w := new(bytes.Buffer)
	e := labgob.NewEncoder(w)
	if e.Encode(kv.kvMap) != nil ||
		e.Encode(kv.validShards) != nil ||
		e.Encode(kv.lastOps) != nil ||
		e.Encode(kv.receivedHandoffs) != nil ||
		e.Encode(kv.pendingHandoffs) != nil {

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
	var validShards map[int]bool
	var lastOps map[int]LastOp

	var receivedHandOffs map[int][][]int
	var pendingHandOffs map[Handoff]bool

	if d.Decode(&kvMap) != nil ||
		d.Decode(&validShards) != nil ||
		d.Decode(&lastOps) != nil ||
		d.Decode(&receivedHandOffs) != nil ||
		d.Decode(&pendingHandOffs) != nil {

		log.Fatalln("readSnapshotL() failed!")
	} else {

		kv.kvMap = kvMap
		kv.validShards = validShards
		kv.lastOps = lastOps

		kv.receivedHandoffs = receivedHandOffs
		kv.pendingHandoffs = pendingHandOffs
	}
}

// HELPER

func (kv *ShardKV) notNewShardsL(configNum int, newShards []int) bool {
	for _, shards := range kv.receivedHandoffs[configNum] {
		if reflect.DeepEqual(shards, newShards) {
			return true
		}
	}
	return false
}

func (kv *ShardKV) log(verbose LogVerbosity, topic LogTopic, format string, a ...interface{}) {
	format = kv.name + ": " + format
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

// RECONFIGURATION

func (kv *ShardKV) addShardsL(configNum int, shards []int, kvMap map[string]string, lastOps map[int]LastOp) {
	kv.reconfigureL(
		&Op{OpType: opAddShards, ConfigNum: configNum, Shards: shards, KVMap: kvMap, LastOps: lastOps})
}

func (kv *ShardKV) removeShardsL(configNum int, shards []int, gid int) {
	kv.reconfigureL(
		&Op{OpType: opRemoveShards, ConfigNum: configNum, Shards: shards, Gid: gid})
}

func (kv *ShardKV) handoffDoneL(handoff Handoff) {
	kv.reconfigureL(
		&Op{OpType: opHandOffDone, Handoff: handoff})
}

func (kv *ShardKV) reconfigureL(op *Op) { // MEMO: no retry
	kv.log(VBasic, TConfig1, "process op %v (reconfigureL)", op)

	op.ClientId = kv.clientId
	op.OpId = kv.opId
	kv.opId++

	// return if op is the same as lastOp
	if lastOp, ok := kv.lastOps[op.ClientId]; ok && op.isSame(lastOp.ClientId, lastOp.OpId) {
		return
	}

	// Send op to raft by rf.Start(), return if not leader
	commandIndex, _, isLeader := kv.rf.Start(*op)
	if !isLeader {
		return
	}

	kv.waitApplyL(commandIndex, op)
}

// START AND TICKER

func (kv *ShardKV) start() {
	kv.lock("start")
	defer kv.unlock("start")

	// get first valid config
	kv.firstConfigL()

	// Use snapshot to recover
	kv.readSnapshotL(kv.persister.ReadSnapshot())

	go kv.receiveApplyMsg()

	go kv.ticker()
}

func (kv *ShardKV) firstConfigL() {
	kv.unlock("start")
	config := kv.ctrler.Query(1)
	for config.Num == 0 {
		time.Sleep(HeartbeatsInterval)
		config = kv.ctrler.Query(1)
	}
	kv.lock("start")

	kv.log(VVerbose, TConfig2, "first config %v", config.Shards)

	for shard, gid := range config.Shards {
		if gid == kv.gid {
			kv.validShards[shard] = true
		}
	}
}

func (kv *ShardKV) ticker() {
	for {
		time.Sleep(HeartbeatsInterval)

		if _, isLeader := kv.rf.GetState(); isLeader {
			kv.checkPendingHandoffs()
			kv.pollConfig()
		}
	}
}

func (kv *ShardKV) pollConfig() {
	config := kv.ctrler.Query(-1)

	kv.lock("pollConfig")
	defer kv.unlock("pollConfig")

	gidShards := map[int][]int{}
	for shard, gid := range config.Shards {
		if kv.validShards[shard] && gid != kv.gid {
			gidShards[gid] = append(gidShards[gid], shard)
		}
	}

	for gid, shards := range gidShards {
		kv.removeShardsL(config.Num, shards, gid)
	}
}

func (kv *ShardKV) checkPendingHandoffs() {
	kv.lock("checkPendingHandoffs")
	defer kv.unlock("checkPendingHandoffs")

	for handoff, _ := range kv.pendingHandoffs {
		args := &HandOffShardsArgs{kv.gid, kv.me,
			handoff.ConfigNum, *handoff.Shards,
			copyKVMap(kv.kvMap), copyLastOps(kv.lastOps)}

		kv.handOffL(args, handoff.Gid)
		kv.handoffDoneL(handoff)
	}
}

func (kv *ShardKV) handOffL(args *HandOffShardsArgs, gid int) {
	kv.unlock("handOffL")
	servers := kv.ctrler.Query(args.ConfigNum).Groups[gid]
	kv.lock("handOffL")

	for {
		for si := 0; si < len(servers); si++ {
			srv := kv.makeEnd(servers[si])
			reply := &HandOffShardsReply{}

			kv.unlock("handOffL")
			ok := srv.Call("ShardKV.HandOffShards", args, reply)
			kv.lock("handOffL")

			if ok && reply.Err == OK {
				return
			}
			// ... not ok, or ErrWrongLeader
		}

		time.Sleep(HeartbeatsInterval)
	}
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

	go kv.start()

	return kv
}
