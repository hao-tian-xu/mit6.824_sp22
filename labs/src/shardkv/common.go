package shardkv

import "fmt"
import . "6.824/util"

const (
	// client
	opPut    = "Put"
	opAppend = "Append"
	opGet    = "Get"

	rpcGet       = "ShardKV.Get"
	rpcPutAppend = "ShardKV.PutAppend"

	// self
	opUpdateConfig = "UpdateConfig"

	// other groups
	opRequestShards

	rpcRequestShards = "ShardKV.RequestShards"
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

func (o OpReply) String() string {
	return fmt.Sprintf("%v %v", o.Err, o.Value)
}

type RequestShardsArgs struct {
	Gid       int
	Shards    []int
	ConfigNum int

	ClientId int
	OpId     int
}

type RequestShardsReply struct {
	Err   Err
	KVMap map[string]string
}
