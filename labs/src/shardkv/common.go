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
		return fmt.Sprintf("%v-%v Put %v: %v", p.ClientId, p.OpId, p.Key, p.Value)
	} else {
		return fmt.Sprintf("%v-%v Append %v: %v", p.ClientId, p.OpId, p.Key, p.Value)
	}
}

type GetArgs struct {
	Key string

	ClientId int
	OpId     int
}

func (g GetArgs) String() string {
	return fmt.Sprintf("%v-%v Get %v", g.ClientId, g.OpId, g.Key)
}

type OpReply struct {
	Err   Err
	Value string
}

func (o OpReply) String() string {
	return fmt.Sprintf("%v %v", o.Err, o.Value)
}

// HANDOFF SHARDS RPC TYPE

type HandOffShardsArgs struct {
	Gid int
	Sid int

	ConfigNum int
	Shards    []int
	KVMap     map[string]string
	LastOps   map[int]LastOp
}

func (h HandOffShardsArgs) String() string {
	return fmt.Sprintf("HandOff %v:%v", h.ConfigNum, h.Shards)
}

type HandOffShardsReply struct {
	Err Err
}

// HELPER

func topic(op *Op) LogTopic {
	topic := TKVServer2
	if op.OpType == opAddShards || op.OpType == opHandOffShards {
		topic = TConfig2
	}
	return topic
}

func copyKVMap(kvMap map[string]string) map[string]string {
	result := map[string]string{}
	for k, v := range kvMap {
		result[k] = v
	}
	return result
}

func copyLastOps(lastOps map[int]LastOp) map[int]LastOp {
	result := map[int]LastOp{}
	for k, v := range lastOps {
		result[k] = v
	}
	return result
}
