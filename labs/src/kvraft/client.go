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
	ck.Lock()
	opId := ck.opId
	ck.opId++
	ck.Unlock()

	return ck.sendOp(opId, key, "", opGet)
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
	ck.Lock()
	opId := ck.opId
	ck.opId++
	ck.Unlock()

	ck.sendOp(opId, key, value, op)
}

func (ck *Clerk) sendOp(opId int, key string, value string, op string) string {
	ck.Lock()
	leaderId := ck.leaderId
	ck.Unlock()

	reply := OpReply{}
	var ok bool

	LogClient(vBasic, tClient, ck.me, "%v %v/%v\n", op, key, value)
	if op == opGet {
		args := GetArgs{key, ck.me, opId}
		ok = ck.servers[leaderId].Call(rpcGet, &args, &reply)
	} else {
		args := PutAppendArgs{key, value, op, ck.me, opId}
		ok = ck.servers[leaderId].Call(rpcPutAppend, &args, &reply)
	}

	if ok {
		if reply.Err == OK {
			return reply.Value
		} else if reply.Err == ErrNoKey {
			return ""
		} else {
			if reply.Err == ErrDuplicate {
				time.Sleep(_LoopInterval)
			} else if reply.Err == ErrWrongLeader {
				ck.findLeader()
			}
			return ck.sendOp(opId, key, value, op)
		}
	} else {
		LogClient(vVerbose, tError, ck.me, "%v %v/%v failed...\n", op, key, value)

		ck.findLeader()
		return ck.sendOp(opId, key, value, op)
	}
}

func (ck *Clerk) Put(key string, value string) {
	ck.PutAppend(key, value, opPut)
}

func (ck *Clerk) Append(key string, value string) {
	ck.PutAppend(key, value, opAppend)
}

func (ck *Clerk) findLeader() {
	ck.Lock()
	ck.leaderId++
	if ck.leaderId >= ck.nServers {
		ck.leaderId = 0
	}
	ck.Unlock()
}
