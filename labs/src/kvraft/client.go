package kvraft

import (
	"6.824/labrpc"
	"sync"
	"time"
)
import "crypto/rand"
import "math/big"

var clientId = _NA

const (
	// RPC call name
	rpcGet       = "KVServer.Get"
	rpcPutAppend = "KVServer.PutAppend"
)

type Clerk struct {
	servers []*labrpc.ClientEnd
	// You will have to modify this struct.
	me       int
	nServers int

	sync.Mutex
	leaderId int
	opId     int
}

func nrand() int64 {
	max := big.NewInt(int64(1) << 62)
	bigx, _ := rand.Int(rand.Reader, max)
	x := bigx.Int64()
	return x
}

func MakeClerk(servers []*labrpc.ClientEnd) *Clerk {
	ck := new(Clerk)
	ck.servers = servers
	// You'll have to add code here.
	ck.me = clientId
	clientId++
	ck.nServers = len(servers)
	ck.leaderId = 0
	ck.opId = 0
	LogClient(vBasic, tClient, ck.me, "new client!\n")
	return ck
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

	// You will have to modify this function.
	var args GetArgs
	var reply GetReply

	ok := ck.sendOp(key, "", opGet, &args, &reply)

	return ck.processReply(ok, reply.Err, "", reply.Value, opGet)
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
	// You will have to modify this function.
	var args PutAppendArgs
	var reply PutAppendReply

	ok := ck.sendOp(key, value, op, &args, &reply)

	ck.processReply(ok, reply.Err, key, value, op)
}

func (ck *Clerk) sendOp(key string, value string, op string, args interface{}, reply interface{}) bool {
	ck.Lock()
	leaderId := ck.leaderId
	opId := ck.opId
	ck.opId++
	ck.Unlock()

	LogClient(vVerbose, tClient, ck.me, "%v %v/%v\n", op, key, value)
	if op == opGet {
		args = &GetArgs{key, ck.me, opId}
		reply = &GetReply{}
		return ck.servers[leaderId].Call(rpcGet, args, reply)
	} else {
		args = &PutAppendArgs{key, value, op, ck.me, opId}
		reply = &PutAppendReply{}
		return ck.servers[leaderId].Call(rpcPutAppend, args, reply)
	}
}

func (ck *Clerk) processReply(ok bool, err Err, key string, value string, op string) string {
	if ok {
		if err == OK {
			return value
		} else if err == ErrNoKey {
			return ""
		} else {
			if err == ErrDuplicate {
				time.Sleep(_LoopInterval)
			} else if err == ErrWrongLeader {
				ck.Lock()
				ck.leaderId++
				if ck.leaderId >= ck.nServers {
					ck.leaderId = 0
				}
				ck.Unlock()
			}

			if op == opGet {
				return ck.Get(key)
			} else {
				ck.PutAppend(key, value, op)
				return ""
			}
		}
	} else {
		LogClient(vVerbose, tError, ck.me, "%v %v/%v failed...\n", op, key, value)
		return ""
	}
}

func (ck *Clerk) Put(key string, value string) {
	ck.PutAppend(key, value, opPut)
}

func (ck *Clerk) Append(key string, value string) {
	ck.PutAppend(key, value, opAppend)
}

func (ck *Clerk) leaderUpdate(oldLeaderId int, newLeaderId int) {
	if oldLeaderId != newLeaderId {
		ck.Lock()
		ck.leaderId = newLeaderId
		ck.Unlock()
	}
}
