package shardkv

//
// client code to talk to a sharded key/value service.
//
// the client first talks to the shardctrler to find out
// the assignment of shards (keys) to groups, and then
// talks to the group that holds the key's shard.
//

import (
	"6.824/labrpc"
	"6.824/shardctrler"
	. "6.824/util"
	"sync"
	"time"
)

var clientId = NA

//
// which shard is a key in?
// please use this function,
// and please do not change it.
//
func key2shard(key string) int {
	shard := 0
	if len(key) > 0 {
		shard = int(key[0])
	}
	shard %= shardctrler.NShards
	return shard
}

// CLERK

type Clerk struct {
	sm      *shardctrler.Clerk
	config  shardctrler.Config
	makeEnd func(string) *labrpc.ClientEnd

	sync.Mutex
	me           int
	groupLeaders map[int]int
	nextOpId     int
}

//
// fetch the current value for a key.
// returns "" if the key does not exist.
// keeps trying forever in the face of all other errors.
// You will have to modify this function.
//
func (ck *Clerk) Get(key string) string {
	ck.lock("Get")
	defer ck.unlock("Get")

	args := &GetArgs{key, ck.me, ck.nextOpId}
	return ck.sendOpL(key, args, rpcGet, args.String())
}

//
// shared by Put and Append.
// You will have to modify this function.
//
func (ck *Clerk) PutAppend(key string, value string, op string) {
	ck.lock("PutAppend")
	defer ck.unlock("PutAppend")

	args := &PutAppendArgs{key, value, op, ck.me, ck.nextOpId}
	ck.sendOpL(key, args, rpcPutAppend, args.String())
}

func (ck *Clerk) sendOpL(key string, args interface{}, rpc string, format string) string {
	ck.log(VBasic, TClient1, "%v send (sendOpL)", format)
	defer ck.log(VBasic, TClient1, "%v return (sendOpL)", format)

	ck.nextOpId++
	return ck._sendOpL(key, args, rpc, format)
}

func (ck *Clerk) _sendOpL(key string, args interface{}, rpc string, format string) string {
	shard := key2shard(key)
	gid := ck.config.Shards[shard]

	if servers, ok := ck.config.Groups[gid]; ok {
		if _, ok := ck.groupLeaders[gid]; !ok {
			ck.groupLeaders[gid] = 0
		}
		leaderId := ck.groupLeaders[gid]
		srv := ck.makeEnd(servers[leaderId])
		reply := &OpReply{}

		ck.unlock("sendOpL")
		ok := srv.Call(rpc, args, reply)
		ck.lock("sendOpL")

		if ok {
			ck.log(VVerbose, TClient2, "reply from %v: %v (_sendOpL)", leaderId, reply)

			if reply.Err == OK || reply.Err == ErrNoKey {
				return reply.Value
			} else if reply.Err == ErrWrongGroup {
				ck.config = ck.sm.Query(NA)
			} else {
				ck.changeLeaderL(gid, len(servers))
			}
		} else {
			ck.changeLeaderL(gid, len(servers))
		}
	} else {
		ck.config = ck.sm.Query(NA)
		ck.log(VVerbose, TTrace, "config: %v", ck.config)
	}

	ck.unlock("_sendOpL-sleep")
	time.Sleep(HeartbeatsInterval)
	ck.lock("_sendOpL-sleep")

	return ck._sendOpL(key, args, rpc, format)
}

func (ck *Clerk) Put(key string, value string) {
	ck.PutAppend(key, value, "Put")
}

func (ck *Clerk) Append(key string, value string) {
	ck.PutAppend(key, value, "Append")
}

// HELPER

func (ck *Clerk) changeLeaderL(gid int, nServers int) {
	ck.groupLeaders[gid]++
	if ck.groupLeaders[gid] >= nServers {
		ck.groupLeaders[gid] = 0
	}
}

func (ck *Clerk) log(verbose LogVerbosity, topic LogTopic, format string, a ...interface{}) {
	format = "KCLNT: " + format

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

func (ck *Clerk) pollConfig() {
	ck.lock("pollConfig")
	defer ck.unlock("pollConfig")

	for {
		config := ck.sm.Query(NA)
		if config.Num > ck.config.Num {

			ck.log(VBasic, TTrace, "new config %v", config.Num)

			ck.config = config
		}

		ck.unlock("pollConfig")
		time.Sleep(HeartbeatsInterval)
		ck.lock("pollConfig")
	}
}

// MAKE

//
// the tester calls MakeClerk.
//
// ctrlers[] is needed to call shardctrler.MakeClerk().
//
// makeEnd(servername) turns a server name from a
// Config.Groups[gid][i] into a labrpc.ClientEnd on which you can
// send RPCs.
//
func MakeClerk(ctrlers []*labrpc.ClientEnd, makeEnd func(string) *labrpc.ClientEnd) *Clerk {
	ck := new(Clerk)
	ck.sm = shardctrler.MakeClerk(ctrlers)
	ck.makeEnd = makeEnd

	clientId++
	ck.me = clientId % 100

	ck.log(VBasic, TClient2, "start client")

	ck.config = ck.sm.Query(NA)

	ck.log(VVerbose, TTrace, "config: %v", ck.config)

	ck.groupLeaders = map[int]int{}
	ck.nextOpId = 0

	return ck
}
