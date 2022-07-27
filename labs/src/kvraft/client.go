package kvraft

import (
	"6.824/labrpc"
	"log"
	"sync"
	"time"
)
import "crypto/rand"
import "math/big"

// VAR, CONST AND TYPE

var clientId = _NA

const (
	// RPC call name
	rpcGet       = "KVServer.Get"
	rpcPutAppend = "KVServer.PutAppend"
)

type Clerk struct {
	servers []*labrpc.ClientEnd
	// You will have to modify this struct.
	sync.Mutex     // lock
	me         int // client id (increasing order)
	nServers   int // number of kvservers
	leaderId   int // assumed leader id
	nextOpId   int // next op id (to identify op)
}

func nrand() int64 {
	max := big.NewInt(int64(1) << 62)
	bigx, _ := rand.Int(rand.Reader, max)
	x := bigx.Int64()
	return x
}

// MAKE CLERK

func MakeClerk(servers []*labrpc.ClientEnd) *Clerk {
	ck := new(Clerk)
	ck.servers = servers
	// You'll have to add code here.
	clientId++
	if clientId >= 100 {
		// personally limit client id to 2 digits
		log.Fatalln("me: too many clients...")
	}
	ck.me = clientId
	ck.nServers = len(servers)
	ck.leaderId = 0
	ck.nextOpId = 0
	LogClient(vBasic, tClient, ck.me, "new client!\n")
	return ck
}

// RPC STUB

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

	// get unique op id
	ck.Lock()
	opId := ck.nextOpId
	ck.nextOpId++
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

	// get unique op id
	ck.Lock()
	opId := ck.nextOpId
	ck.nextOpId++
	ck.Unlock()

	ck.sendOp(opId, key, value, op)
}

//
// send RPC and process its reply
//
func (ck *Clerk) sendOp(opId int, key string, value string, op string) string {
	ck.Lock()
	leaderId := ck.leaderId
	ck.Unlock()

	// RPC Call
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

	// process RPC reply
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

// HELPER

//
// change leaderId to retry, TODO: change the way to find leader
//
func (ck *Clerk) findLeader() {
	ck.Lock()
	ck.leaderId++
	if ck.leaderId >= ck.nServers {
		ck.leaderId = 0
	}
	ck.Unlock()
}
