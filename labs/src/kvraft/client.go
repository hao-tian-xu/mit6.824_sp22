package kvraft

import (
	"6.824/labrpc"
	"sync"
	"time"
)
import . "6.824/util"

// VAR, CONST AND TYPE

var clientId = NA

// CLERK

type Clerk struct {
	servers []*labrpc.ClientEnd
	// You will have to modify this struct.
	sync.Mutex        // lock
	me            int // client id (increasing order)
	currentLeader int // assumed leader id
	nextOpId      int // next op id (to identify op)
}

//
// fetch the current value for a key.
// returns "" if the key does not exist.
// keeps trying forever in the face of all other errors.
//
// you can send an RPC with code like this:
// ok := ck.servers[i].Call("KVServer.Get", &args, &reply)
//
// the types of args and reply (including whether they are pointers)
// must match the declared types of the RPC handler function's
// arguments. and reply must be passed as a pointer.
//
func (ck *Clerk) Get(key string) string {
	ck.lock("Get")
	defer ck.unlock("Get")

	args := &GetArgs{key, ck.me, ck.nextOpId}
	return ck.sendOpL(args, rpcGet, args.String())
}

//
// shared by Put and Append.
//
// you can send an RPC with code like this:
// ok := ck.servers[i].Call("KVServer.PutAppend", &args, &reply)
//
// the types of args and reply (including whether they are pointers)
// must match the declared types of the RPC handler function's
// arguments. and reply must be passed as a pointer.
//
func (ck *Clerk) PutAppend(key string, value string, op string) {
	ck.lock("PutAppend")
	defer ck.unlock("PutAppend")

	args := &PutAppendArgs{key, value, op, ck.me, ck.nextOpId}
	ck.sendOpL(args, rpcPutAppend, args.String())
}

func (ck *Clerk) sendOpL(args interface{}, rpc string, format string) string {
	ck.log(VBasic, TClient1, "%v send to %v (sendOpL)", format, ck.currentLeader)
	defer ck.log(VBasic, TClient1, "%v return from %v (sendOpL)", format, ck.currentLeader)

	ck.nextOpId++
	return ck._sendOpL(args, rpc, format)
}

func (ck *Clerk) _sendOpL(args interface{}, rpc string, format string) string {
	leaderId := ck.currentLeader
	reply := &OpReply{}

	ck.unlock("_sendOpL")
	ok := ck.servers[leaderId].Call(rpc, args, reply)
	ck.lock("_sendOpL")

	if ok {
		ck.log(VVerbose, TClient2, "reply from %v: %v (_sendOpL)", leaderId, reply)

		if reply.Err == OK {
			return reply.Value
		}
		if reply.Err == ErrNoKey {
			return ""
		}
	}

	ck.changeLeaderL()
	return ck._sendOpL(args, rpc, format)
}

func (ck *Clerk) Put(key string, value string) {
	ck.PutAppend(key, value, opPut)
}

func (ck *Clerk) Append(key string, value string) {
	ck.PutAppend(key, value, opAppend)
}

// HELPER

func (ck *Clerk) changeLeaderL() {
	ck.currentLeader++
	if ck.currentLeader >= len(ck.servers) {
		ck.currentLeader = 0
	}

	ck.unlock("changeLeaderL")
	time.Sleep(HeartbeatsInterval)
	ck.lock("changeLeaderL")
}

func (ck *Clerk) log(verbose LogVerbosity, topic LogTopic, format string, a ...interface{}) {
	LogKVClnt(verbose, topic, ck.me, format, a...)
}

func (ck *Clerk) lock(method string) {
	ck.log(VExcessive, TTrace, "acquire lock (%v)", method)
	ck.Lock()
}

func (ck *Clerk) unlock(method string) {
	ck.log(VExcessive, TTrace, "release lock (%v)", method)
	ck.Unlock()
}

// MAKE CLERK

func MakeClerk(servers []*labrpc.ClientEnd) *Clerk {
	ck := new(Clerk)
	ck.servers = servers

	clientId++
	ck.me = clientId % 100 // personally limit client id to 2 digits

	ck.log(VBasic, TClient2, "start client")

	ck.currentLeader = 0
	ck.nextOpId = 0

	return ck
}
