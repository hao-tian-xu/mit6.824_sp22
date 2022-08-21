package kvraft

import "fmt"
import . "6.824/util"

// CONST AND TYPE

const (
	opPut    = "Put"
	opAppend = "Append"
	opGet    = "Get"

	rpcGet       = "KVServer.Get"
	rpcPutAppend = "KVServer.PutAppend"
)

// RPC ARGS AND REPLY

//
// Put or Append
//
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
