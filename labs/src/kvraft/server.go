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
	_LoopInterval = 10 * time.Millisecond
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
	kvMap              map[string]string // kv storage
	processingOpMap    map[Op]bool       // save Ops that are being processed, TODO: maybe unnecessary
	lastProcessedOpMap map[int]Op        // save last processed Op for eache client, TODO: also remember result
	// channels for ApplyMsg transfer
	applyOpChans map[int]chan Op
}

// RPC

func (kv *KVServer) Get(args *GetArgs, reply *OpReply) {
	// Your code here.
	op := Op{OpType: opGet, Key: args.Key, ClientId: args.ClientId, OpId: args.OpId}

	reply.Err, reply.Value = kv.processOp(opGet, &op)
}

func (kv *KVServer) PutAppend(args *PutAppendArgs, reply *OpReply) {
	// Your code here.
	op := Op{OpType: args.Op, Key: args.Key, Value: args.Value, ClientId: args.ClientId, OpId: args.OpId}
	reply.Err, _ = kv.processOp(args.Op, &op)
}

//
// process RPC request
//
func (kv *KVServer) processOp(opType string, op *Op) (Err, string) {
	// check isLeader before processing or porcessed check, TODO: modify find leader method (may make this unnecessary)
	if _, isLeader := kv.rf.GetState(); !isLeader {
		LogKV(vVerbose, tTrace, kv.me, "notLeader... (%v)\n", opType)
		return ErrWrongLeader, ""
	}

	kv.mu.Lock()
	if kv.processingOpMap[*op] {
		// op being processed, TODO: may be unnecessary
		kv.mu.Unlock()
		LogKV(vBasic, tTrace, kv.me, "processing... (%v)\n", opType)
		return ErrDuplicate, ""
	} else if opType != opGet && kv.lastProcessedOpMap[op.ClientId] == *op {
		// just processed, TODO: also for Get operation (also save op result (a new struct for map value))
		kv.mu.Unlock()
		LogKV(vBasic, tTrace, kv.me, "processed... (%v)\n", opType)
		return OK, ""
	}

	// Send op to raft by rf.Start()
	commandIndex, _, isLeader := kv.rf.Start(*op)
	//		if not isLeader
	if !isLeader {
		kv.mu.Unlock()
		LogKV(vVerbose, tTrace, kv.me, "notLeader... (%v)\n", opType)
		return ErrWrongLeader, ""
	}

	// make op as being processed, TODO: may be unnecessary
	kv.processingOpMap[*op] = true

	// make a channel for Op transfer, if not existing
	if _, ok := kv.applyOpChans[commandIndex]; !ok {
		kv.applyOpChans[commandIndex] = make(chan Op)
	}

	kv.mu.Unlock()

	// Wait for the op to be applied
	applied := kv.waitApplyMsg(commandIndex, op)

	kv.mu.Lock()
	// already processed or failed, delete from processing, TODO: may be unnecessary
	delete(kv.processingOpMap, *op)

	value, ok := kv.kvMap[op.Key]
	kv.mu.Unlock()

	// Reply to client (and log)
	if applied {
		switch opType {
		case opGet:
			if ok {
				LogKV(vBasic, tKVServer, kv.me, "get %v/%v! (Get)\n", op.Key, value)
				return OK, value
			} else {
				LogKV(vBasic, tKVServer, kv.me, "get %v/nokey... (Get)\n", op.Key)
				return ErrNoKey, ""
			}
		case opPut:
			LogKV(vBasic, tKVServer, kv.me, "put %v/%v! (Put)\n", op.Key, op.Value)
			return OK, ""
		case opAppend:
			if ok {
				LogKV(vBasic, tKVServer, kv.me, "append %v/%v! (Append)\n", op.Key, op.Value)
			} else {
				LogKV(vBasic, tKVServer, kv.me, "put %v/%v! (Append)\n", op.Key, op.Value)
			}
			return OK, ""
		default:
			log.Fatalln("unkown opType!! (processOp)")
			return ErrFatal, ""
		}
	} else {
		LogKV(vBasic, tKVServer, kv.me, "%v notCommited... (processOp)\n", opType)
		return ErrNotApplied, ""
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

// APPLYMSG

//
// wait for the commandIndex to be commited in raft, return true if it's the same op
//
func (kv *KVServer) waitApplyMsg(commandIndex int, op *Op) bool {
	kv.mu.Lock()
	ch := kv.applyOpChans[commandIndex]
	kv.mu.Unlock()
	// wait for the commandIndex to be commited in raft and applied in kvserver
	appliedOp := <-ch

	// return true if it's the same op
	if appliedOp == *op {
		LogKV(vVerbose, tTrace, kv.me, "command%v applied!", commandIndex)
		return true
	} else {
		return false
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
				if lastOp, started := kv.lastProcessedOpMap[op.ClientId]; !started || op != lastOp {
					// Apply the command to kvserver
					switch op.OpType {
					case opPut:
						kv.kvMap[op.Key] = op.Value
					case opAppend:
						_, exist := kv.kvMap[op.Key]
						if exist {
							kv.kvMap[op.Key] += op.Value
						} else {
							kv.kvMap[op.Key] = op.Value
						}
					}
					// remember last processed op
					kv.lastProcessedOpMap[op.ClientId] = op

					if op.OpType != opGet {
						LogKV(vVerbose, tTrace, kv.me, "%v %v/%v applied! (receiveApplyMsg)", op.OpType, op.Key, op.Value)
					}
				} else {
					// command already processed
					LogKV(vVerbose, tTrace, kv.me, "%v %v/%v already processed... (receiveApplyMsg)", op.OpType, op.Key, op.Value)
				}

				// Send Op to RPC if relevant channel exists
				ch, ok := kv.applyOpChans[applyMsg.CommandIndex]
				kv.mu.Unlock()
				if ok {
					ch <- op
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
	kv.processingOpMap = map[Op]bool{}
	kv.lastProcessedOpMap = map[int]Op{}

	kv.applyOpChans = map[int]chan Op{}

	// You may need initialization code here.
	// Start receiving ApplyMsg from raft
	go kv.receiveApplyMsg()

	return kv
}
