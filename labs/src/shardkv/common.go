package shardkv

import "fmt"
import . "6.824/util"

const (
	opPut    = "Put"
	opAppend = "Append"
	opGet    = "Get"

	rpcGet       = "ShardKV.Get"
	rpcPutAppend = "ShardKV.PutAppend"

	opAddShards     = "AddShards"
	opHandOffShards = "HandOffShards"
)

//
// Sharded key/value server.
// Lots of replica groups, each running Raft.
// Shardctrler decides which group serves each shard.
// Shardctrler may change shard assignment from time to time.
//
// You will have to modify these definitions.
//

// Put or Append
type PutAppendArgs struct {
	Key   string
	Value string
	Op    string // "Put" or "Append"

	ClientId int
	OpId     int
}

func (p PutAppendArgs) String() string {
	if p.Op == opPut {
		return fmt.Sprintf("Put%v-%v %v:%v", p.ClientId, p.OpId, p.Key, p.Value)
	} else {
		return fmt.Sprintf("Append%v-%v %v:%v", p.ClientId, p.OpId, p.Key, p.Value)
	}
}

type GetArgs struct {
	Key string

	ClientId int
	OpId     int
}

func (g GetArgs) String() string {
	return fmt.Sprintf("Get%v-%v %v", g.ClientId, g.OpId, g.Key)
}

type OpReply struct {
	Err   Err
	Value string
}

// HANDOFF SHARDS RPC TYPE

type HandOffShardsArgs struct {
	Gid     int
	Shards  []int
	KVMap   map[string]string
	LastOps map[int]LastOp
}

func (h HandOffShardsArgs) String() string {
	return fmt.Sprintf("HandOff %v", h.Shards)
}

type HandOffShardsReply struct {
	Err Err
}
