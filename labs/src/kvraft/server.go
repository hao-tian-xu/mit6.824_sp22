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
	kvMap              map[string]string
	applyMsgMap        map[int]interface{}
	processingOpMap    map[Op]bool
	lastProcessedOpMap map[int]Op
}

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

func (kv *KVServer) processOp(opType string, op *Op) (Err, string) {
	if _, isLeader := kv.rf.GetState(); !isLeader {
		LogKV(vVerbose, tTrace, kv.me, "notLeader... (%v)\n", opType)
		return ErrWrongLeader, ""
	}
	kv.mu.Lock()
	if kv.processingOpMap[*op] {
		kv.mu.Unlock()
		LogKV(vBasic, tTrace, kv.me, "processing... (%v)\n", opType)
		return ErrDuplicate, ""
	} else if opType != opGet && kv.lastProcessedOpMap[op.ClientId] == *op {
		kv.mu.Unlock()
		LogKV(vBasic, tTrace, kv.me, "processed... (%v)\n", opType)
		return OK, ""
	}
	commandIndex, _, isLeader := kv.rf.Start(*op)
	kv.processingOpMap[*op] = true
	kv.mu.Unlock()

	if !isLeader {
		LogKV(vVerbose, tTrace, kv.me, "notLeader... (%v)\n", opType)
		return ErrWrongLeader, ""
	}

	commited := kv.waitApply(commandIndex, op)

	if commited {
		kv.mu.Lock()
		delete(kv.processingOpMap, *op)
		//kv.lastProcessedOpMap[op.ClientId] = *op
		value, ok := kv.kvMap[op.Key]
		kv.mu.Unlock()

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
			//kv.kvMap[op.Key] = op.Value
			LogKV(vBasic, tKVServer, kv.me, "put %v/%v! (Put)\n", op.Key, op.Value)
			return OK, ""
		case opAppend:
			if ok {
				//kv.kvMap[op.Key] += op.Value
				LogKV(vBasic, tKVServer, kv.me, "append %v/%v! (Append)\n", op.Key, op.Value)
			} else {
				//kv.kvMap[op.Key] = op.Value
				LogKV(vBasic, tKVServer, kv.me, "put %v/%v! (Append)\n", op.Key, op.Value)
			}
			return OK, ""
		default:
			log.Fatalln("unkown opType!! (processOp)")
			return ErrFatal, ""
		}
	} else {
		LogKV(vBasic, tKVServer, kv.me, "%v notCommited... (processOp)\n", opType)
		return ErrNotCommited, ""
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
	var command interface{}
	var ok bool

	for {
		kv.mu.Lock()
		command, ok = kv.applyMsgMap[commandIndex]
		kv.mu.Unlock()
		if ok {
			break
		}
		time.Sleep(_LoopInterval)
	}

	if command == *op {
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
			kv.mu.Lock()
			kv.applyMsgMap[applyMsg.CommandIndex] = applyMsg.Command

			if op, ok := applyMsg.Command.(Op); ok {
				if _, ok = kv.lastProcessedOpMap[op.ClientId]; !ok ||
					op != kv.lastProcessedOpMap[op.ClientId] {
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
					kv.lastProcessedOpMap[op.ClientId] = op
					if op.OpType != opGet {
						LogKV(vBasic, tTrace, kv.me, "%v %v/%v applied! (receiveApplyMsg)", op.OpType, op.Key, op.Value)
					}
				}
			} else {
				log.Fatalln("command not op!!")
			}
			kv.mu.Unlock()
		}
	}
}

func (kv *KVServer) deleteApplyMsg(commandIndex int) {
	kv.mu.Lock()
	delete(kv.applyMsgMap, commandIndex)
	kv.mu.Unlock()
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
	kv.applyMsgMap = map[int]interface{}{}
	kv.processingOpMap = map[Op]bool{}
	kv.lastProcessedOpMap = map[int]Op{}

	// You may need initialization code here.
	go kv.receiveApplyMsg()

	return kv
}
