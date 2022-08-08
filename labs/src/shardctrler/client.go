package shardctrler

//
// Shardctrler clerk.
//

import (
	"6.824/labrpc"
	. "6.824/util"
	"fmt"
	"sync"
	"time"
)
import "crypto/rand"
import "math/big"

var clientId = NA

const (
	rpcQuery = "ShardCtrler.Query"
	rpcJoin  = "ShardCtrler.Join"
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
	LogCtrlerClnt(Basic, Client, ck.me, "new client!")
	return ck
}

func (ck *Clerk) Join(servers map[int][]string) {
	// Your code here.
	ck.Lock()
	opId := ck.nextOpId
	ck.nextOpId++
	ck.Unlock()

	args := &JoinArgs{Servers: servers,
		ClientId: ck.me, OpId: opId}

	ck._sendOp(opId, args, rpcJoin,
		fmt.Sprintf("Join%v-%v %v", ck.me, opId, fmt.Sprint(groupsGids(servers))))
}

func (ck *Clerk) Leave(gids []int) {
	// Your code here.
	ck.Lock()
	opId := ck.nextOpId
	ck.nextOpId++
	ck.Unlock()

	args := &LeaveArgs{GIDs: gids,
		ClientId: ck.me, OpId: opId}

	ck._sendOp(opId, args, rpcLeave,
		fmt.Sprintf("Leave%v-%v %v", ck.me, opId, fmt.Sprint(gids)))
}

func (ck *Clerk) Move(shard int, gid int) {
	// Your code here.
	ck.Lock()
	opId := ck.nextOpId
	ck.nextOpId++
	ck.Unlock()

	args := &MoveArgs{Shard: shard, GID: gid,
		ClientId: ck.me, OpId: opId}

	ck._sendOp(opId, args, rpcMove,
		fmt.Sprintf("Move%v-%v %v->%v", ck.me, opId, shard, gid))
}

func (ck *Clerk) Query(num int) Config {
	// Your code here.
	ck.Lock()
	opId := ck.nextOpId
	ck.nextOpId++
	ck.Unlock()

	args := &QueryArgs{Num: num,
		ClientId: ck.me, OpId: opId}

	return ck._sendOp(opId, args, rpcQuery,
		fmt.Sprintf("Query%v-%v %v", ck.me, opId, num))
}

func (ck *Clerk) _sendOp(opId int, args interface{}, rpc string, format string) Config {
	LogCtrlerClnt(Basic, Client, ck.me, "%v send!", format)
	defer LogCtrlerClnt(Basic, Client, ck.me, "%v return!", format)
	return ck.sendOp(opId, args, rpc)
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
		switch reply.Err {
		case OK:
			return reply.Config
		case ErrWrongLeader:
			ck.changeLeader(reply.CurrentLeader)
			LogCtrlerClnt(Verbose, Redo, ck.me, "resend: wrong leader... (sendOp)")
		case ErrTimeout:
			ck.changeLeader(NA)
			LogCtrlerClnt(Verbose, Redo, ck.me, "resend: timeout... (sendOp)")
		default:
			LogCtrlerClnt(Verbose, Redo, ck.me, "resend: not applied... (sendOp)")
		}
	} else {
		// Send RPC failed: retry
		ck.changeLeader(NA)
		LogCtrlerClnt(Verbose, Error, ck.me, "resend: send failed... (sendOp)")
	}
	return ck.sendOp(opId, args, rpc)
}

// HELPER

func (ck *Clerk) changeLeader(leader int) {
	ck.Lock()
	if leader == NA || leader == ck.currentLeader {
		ck.currentLeader++
		if ck.currentLeader >= ck.nServers {
			ck.currentLeader = 0
		}
		ck.Unlock()
		time.Sleep(HeartBeatsInterval)
	} else {
		ck.currentLeader = leader
		ck.Unlock()
	}
}
