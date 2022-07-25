package kvraft

import (
	"6.824/labrpc"
	"os"
	"sync"
	"time"
)
import "crypto/rand"
import "math/big"

const (
	// RPC call name
	rpcGet       = "KVServer.Get"
	rpcPutAppend = "KVServer.PutAppend"

	// Op name
	opPut    = "Put"
	opAppend = "Append"
)

type Clerk struct {
	servers []*labrpc.ClientEnd
	// You will have to modify this struct.
	pid      int
	nServers int

	sync.Mutex
	leaderId int
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
	ck.pid = os.Getpid() % 10
	ck.nServers = len(servers)
	ck.leaderId = 0
	LogKV(vBasic, tClient, ck.pid, "new client!\n")
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
	LogKV(vVerbose, tClient, ck.pid, "Clerk.Get\n")
	var args GetArgs
	var reply GetReply

	ck.Lock()
	oldLeaderId, leaderId := ck.leaderId, ck.leaderId
	ck.Unlock()
	for {
		args = GetArgs{key}
		reply = GetReply{}
		ok := ck.servers[leaderId].Call(rpcGet, &args, &reply)
		if ok {
			switch reply.Err {
			case OK:
				ck.leaderUpdate(oldLeaderId, leaderId)
				return reply.Value
			case ErrNoKey:
				ck.leaderUpdate(oldLeaderId, leaderId)
				return ""
			case ErrWrongLeader:
				leaderId++
				if leaderId >= ck.nServers {
					leaderId = 0
				}
			}
		}
		time.Sleep(_LoopInterval)
	}
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
	LogKV(vVerbose, tClient, ck.pid, "Clerk.PutAppend\n")
	var args PutAppendArgs
	var reply PutAppendReply

	ck.Lock()
	oldLeaderId, leaderId := ck.leaderId, ck.leaderId
	ck.Unlock()
	for {
		args = PutAppendArgs{key, value, op}
		reply = PutAppendReply{}
		ok := ck.servers[leaderId].Call(rpcPutAppend, &args, &reply)
		if ok {
			switch reply.Err {
			case OK:
				ck.leaderUpdate(oldLeaderId, leaderId)
				return
			case ErrWrongLeader:
				leaderId++
				if leaderId >= ck.nServers {
					leaderId = 0
				}
			}
		}
		time.Sleep(_LoopInterval)
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
