package shardctrler

//
// Shardctrler clerk.
//

import (
	"6.824/labrpc"
	"sync"
	"time"
)
import . "6.824/util"

var clientId = NA

// CLERK

type Clerk struct {
	servers []*labrpc.ClientEnd
	// Your data here.
	sync.Mutex        // lock
	me            int // client id (increasing order)
	currentLeader int // assumed leader id
	nextOpId      int // next op id (to identify op)
}

func (ck *Clerk) Join(servers map[int][]string) {
	// Your code here.
	ck.lock("Join")
	defer ck.unlock("Join")

	args := &JoinArgs{servers, ck.me, ck.nextOpId}
	ck.sendOpL(args, rpcJoin, args.String())
}

func (ck *Clerk) Leave(gids []int) {
	// Your code here.
	ck.lock("Leave")
	defer ck.unlock("Leave")

	args := &LeaveArgs{gids, ck.me, ck.nextOpId}
	ck.sendOpL(args, rpcLeave, args.String())
}

func (ck *Clerk) Move(shard int, gid int) {
	// Your code here.
	ck.lock("Move")
	defer ck.unlock("Move")

	args := &MoveArgs{shard, gid, ck.me, ck.nextOpId}
	ck.sendOpL(args, rpcMove, args.String())
}

func (ck *Clerk) Query(num int) Config {
	// Your code here.
	ck.lock("Query")
	defer ck.unlock("Query")

	args := &QueryArgs{num, ck.me, ck.nextOpId}
	return ck.sendOpL(args, rpcQuery, args.String())
}

func (ck *Clerk) sendOpL(args interface{}, rpc string, format string) Config {
	ck.log(VBasic, TClient1, "%v send! (sendOpL)", format)
	defer ck.log(VBasic, TClient1, "%v return! (sendOpL)", format)

	ck.nextOpId++
	return ck._sendOpL(args, rpc, format)
}

func (ck *Clerk) _sendOpL(args interface{}, rpc string, format string) Config {
	leaderId := ck.currentLeader
	reply := &OpReply{}

	ck.unlock("_sendOpL")
	ok := ck.servers[leaderId].Call(rpc, args, reply)
	ck.lock("_sendOpL")

	if ok {
		ck.log(VVerbose, TClient2, "reply from %v: %v (_sendOpL)", leaderId, reply)

		if reply.Err == OK {
			return reply.Config
		}
	}

	ck.changeLeaderL()
	return ck._sendOpL(args, rpc, format)
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
	LogCtrlerClnt(verbose, topic, ck.me, format, a...)
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
	// Your code here.
	ck.log(VBasic, TClient2, "start client")

	clientId++
	ck.me = clientId % 100 // personally limit client id to 2 digits
	ck.currentLeader = 0
	ck.nextOpId = 0
	return ck
}
