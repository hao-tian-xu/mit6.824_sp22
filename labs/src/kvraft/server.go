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

	tClient   LogTopic = "CLNT"
	tKVServer LogTopic = "KVSR"

	// timing
	_LoopInterval = 10 * time.Millisecond
)

const Debug = false

func DPrintf(format string, a ...interface{}) (n int, err error) {
	if Debug {
		log.Printf(format, a...)
	}
	return
}

type Op struct {
	// Your definitions here.
	// Field names must start with capital letters,
	// otherwise RPC will break.
	Key   string
	Value string
	// 3A Hint: You will need to uniquely identify client operations
	ServerId int
	OpId     int
}

type OpType int

type KVServer struct {
	mu      sync.Mutex
	me      int
	rf      *raft.Raft
	applyCh chan raft.ApplyMsg
	dead    int32 // set by Kill()

	maxraftstate int // snapshot if log grows this big

	// Your definitions here.
	kvMap       map[string]string
	condLock    sync.Mutex
	cond        *sync.Cond
	applyMsgMap ApplyMsgMap
	// 3A Hint: You will need to uniquely identify client operations
	nextOpId int
}

type ApplyMsgMap struct {
	sync.Mutex
	m map[int]*interface{}
}

func (kv *KVServer) Get(args *GetArgs, reply *GetReply) {
	// Your code here.
	kv.mu.Lock()
	opId := kv.nextOpId
	kv.nextOpId++
	kv.mu.Unlock()
	op := Op{Key: args.Key, ServerId: kv.me, OpId: opId}

	commandIndex, _, isLeader := kv.rf.Start(op)
	if !isLeader {
		reply.Err = ErrWrongLeader
		LogKV(vBasic, tKVServer, kv.me, "notLeader... (Get)\n")
		return
	}

	ok := kv.waitApply(commandIndex, &op)
	if ok {
		kv.mu.Lock()
		value, ok := kv.kvMap[args.Key]
		kv.mu.Unlock()
		if ok {
			reply.Value = value
			reply.Err = OK
			LogKV(vBasic, tKVServer, kv.me, "get %v/%v! (Get)\n", args.Key, value)
		} else {
			reply.Err = ErrNoKey
			LogKV(vBasic, tKVServer, kv.me, "get %v/nokey... (Get)\n", args.Key)
		}
		kv.deleteApplyMsg(commandIndex)
	} else {
		LogKV(vBasic, tKVServer, kv.me, "Get failed... (Get)\n")
	}
}

func (kv *KVServer) PutAppend(args *PutAppendArgs, reply *PutAppendReply) {
	// Your code here.
	kv.mu.Lock()
	opId := kv.nextOpId
	kv.nextOpId++
	kv.mu.Unlock()
	op := Op{Key: args.Key, Value: args.Value, ServerId: kv.me, OpId: opId}

	commandIndex, _, isLeader := kv.rf.Start(op)
	if !isLeader {
		reply.Err = ErrWrongLeader
		LogKV(vBasic, tKVServer, kv.me, "notLeader... (PutAppend)\n")
		return
	}

	ok := kv.waitApply(commandIndex, &op)
	if ok {
		kv.mu.Lock()
		if args.Op == opPut {
			kv.kvMap[args.Key] = args.Value
			LogKV(vBasic, tKVServer, kv.me, "put %v/%v! (Put)\n", args.Key, args.Value)
		} else if _, ok := kv.kvMap[args.Key]; ok {
			kv.kvMap[args.Key] += args.Value
			LogKV(vBasic, tKVServer, kv.me, "append %v/%v! (Append)\n", args.Key, args.Value)
		} else {
			kv.kvMap[args.Key] = args.Value
			LogKV(vBasic, tKVServer, kv.me, "put %v/%v! (Append)\n", args.Key, args.Value)
		}
		kv.mu.Unlock()

		reply.Err = OK
		kv.deleteApplyMsg(commandIndex)
	} else {
		LogKV(vBasic, tKVServer, kv.me, "PutAppend failed... (PutAppend)\n")
	}
}

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

// HELPER METHOD

func (kv *KVServer) waitApply(commandIndex int, op *Op) bool {
	var command *interface{}
	var ok bool

	for {
		kv.applyMsgMap.Lock()
		command, ok = kv.applyMsgMap.m[commandIndex]
		kv.applyMsgMap.Unlock()
		if ok {
			break
		}
		time.Sleep(_LoopInterval)
	}

	if *command == *op {
		LogKV(vExcessive, tTrace, kv.me, "command%v applied!", commandIndex)
		return true
	} else {
		return false
	}
}

func (kv *KVServer) receiveApplyMsg() {
	var applyMsg raft.ApplyMsg
	for !kv.killed() {
		applyMsg = <-kv.applyCh
		if applyMsg.CommandValid {
			kv.applyMsgMap.Lock()
			kv.applyMsgMap.m[applyMsg.CommandIndex] = &applyMsg.Command
			kv.applyMsgMap.Unlock()
		}
	}
}

func (kv *KVServer) deleteApplyMsg(commandIndex int) {
	kv.cond.L.Lock()
	delete(kv.applyMsgMap.m, commandIndex)
	kv.cond.L.Unlock()
}

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
	LogKV(vExcessive, tKVServer, kv.me, "new kvServer!")

	kv.applyCh = make(chan raft.ApplyMsg)
	kv.rf = raft.Make(servers, me, persister, kv.applyCh)

	kv.kvMap = map[string]string{}

	kv.cond = sync.NewCond(&kv.condLock)
	kv.applyMsgMap = ApplyMsgMap{m: map[int]*interface{}{}}

	kv.nextOpId = 0

	// You may need initialization code here.
	go kv.receiveApplyMsg()

	return kv
}
