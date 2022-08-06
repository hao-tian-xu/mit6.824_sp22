package shardctrler

//
// Shardctrler clerk.
//

import (
	"6.824/debug"
	"6.824/labrpc"
	"sync"
)
import "time"
import "crypto/rand"
import "math/big"

var clientId = _NA

const (
	rpcQuery = "ShardCtrler.Query"
	rpcJoint = "ShardCtrler.Join"
	rpcLeave = "ShardCtrler.Leave"
	rpcMove  = "ShardCtrler.Move"
)

type Clerk struct {
	servers []*labrpc.ClientEnd
	// Your data here.
	sync.Mutex        // lock
	me            int // client id (increasing order)
	nServers      int // number of kvservers
	currentLeader int // assumed leader id
	nextOpId      int // next op id (to identify op)
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
	// Your code here.
	clientId++
	ck.me = clientId % 100 // personally limit client id to 2 digits
	ck.nServers = len(servers)
	ck.currentLeader = 0
	ck.nextOpId = 0
	debug.CtrlerClnt(vBasic, tClient, ck.me, "new client!\n")
	return ck
}

func (ck *Clerk) Query(num int) Config {
	args := &QueryArgs{}
	// Your code here.
	args.Num = num

	ck.Lock()
	opId := ck.nextOpId
	ck.nextOpId++
	ck.Unlock()

	return ck.sendOp(opId, args, rpcQuery)
}

func (ck *Clerk) Join(servers map[int][]string) {
	args := &JoinArgs{}
	// Your code here.
	args.Servers = servers

	ck.Lock()
	opId := ck.nextOpId
	ck.nextOpId++
	ck.Unlock()

	ck.sendOp(opId, args, rpcQuery)
}

func (ck *Clerk) Leave(gids []int) {
	args := &LeaveArgs{}
	// Your code here.
	args.GIDs = gids

	ck.Lock()
	opId := ck.nextOpId
	ck.nextOpId++
	ck.Unlock()

	ck.sendOp(opId, args, rpcQuery)
}

func (ck *Clerk) Move(shard int, gid int) {
	args := &MoveArgs{}
	// Your code here.
	args.Shard = shard
	args.GID = gid

	ck.Lock()
	opId := ck.nextOpId
	ck.nextOpId++
	ck.Unlock()

	ck.sendOp(opId, args, rpcQuery)
}

func (ck *Clerk) sendOp(opId int, args interface{}, rpc string) Config {
	ck.Lock()
	leaderId := ck.currentLeader
	ck.Unlock()

	// RPC Call
	reply := &OpReply{}
	ok := ck.servers[leaderId].Call(rpc, args, reply)

	// Process RPC reply
	if ok {
		// Send RPC successed
		if reply.Err == OK {
			return reply.Config
		} else {
			// other conditions: retry
			if reply.Err == ErrWrongLeader {
				if reply.CurrentLeader == _NA {
					// server doesn't know currentLeader
					ck.randomServer()
					time.Sleep(_HeartBeatInterval)
				} else {
					ck.Lock()
					if reply.CurrentLeader != ck.currentLeader {
						ck.currentLeader = reply.CurrentLeader
						ck.Unlock()
					} else {
						ck.Unlock()
						ck.randomServer()
					}
				}
			}
		}
	} else {
		// Send RPC failed: retry
		ck.randomServer()
		time.Sleep(_HeartBeatInterval)
	}
	return ck.sendOp(opId, args, rpc)
}

// HELPER

//
// change currentLeader to retry
//
func (ck *Clerk) randomServer() {
	ck.Lock()
	ck.currentLeader++
	if ck.currentLeader >= ck.nServers {
		ck.currentLeader = 0
	}
	ck.Unlock()
}
