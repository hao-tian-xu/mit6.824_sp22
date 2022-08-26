package shardctrler

import "fmt"
import . "6.824/util"

//
// Shard controler: assigns shards to replication groups.
//
// RPC interface:
// Join(servers) -- add a set of groups (gid -> server-list mapping).
// Leave(gids) -- delete a set of groups.
// Move(shard, gid) -- hand off one shard from current owner to gid.
// Query(num) -> fetch Config # num, or latest config if num==-1.
//
// A Config (configuration) describes a set of replica groups, and the
// replica group responsible for each shard. Configs are numbered. Config
// #0 is the initial configuration, with no groups and all shards
// assigned to group 0 (the invalid group).
//
// You will need to add fields to the RPC argument structs.
//

// The number of shards.
const NShards = 10

// OP CONST

const (
	opQuery = "Query"
	opJoin  = "Join"
	opLeave = "Leave"
	opMove  = "Move"

	rpcQuery = "ShardCtrler.Query"
	rpcJoin  = "ShardCtrler.Join"
	rpcLeave = "ShardCtrler.Leave"
	rpcMove  = "ShardCtrler.Move"
)

// CONFIG

// A configuration -- an assignment of shards to groups.
// Please don't change this.
type Config struct {
	Num    int              // config number
	Shards [NShards]int     // shard -> gid
	Groups map[int][]string // gid -> servers[]
}

func (c Config) String() string {
	return fmt.Sprintf("#%v, %v, %v", c.Num, c.Shards, groupsToGids(c.Groups))
}

// RPC TYPES

type JoinArgs struct {
	Servers map[int][]string // new GID -> servers mappings

	ClientId int
	OpId     int
}

func (j JoinArgs) String() string {
	return fmt.Sprintf("Join%v-%v %v", j.ClientId, j.OpId, fmt.Sprint(groupsToGids(j.Servers)))
}

type LeaveArgs struct {
	GIDs []int

	ClientId int
	OpId     int
}

func (l LeaveArgs) String() string {
	return fmt.Sprintf("Leave%v-%v %v", l.ClientId, l.OpId, fmt.Sprint(l.GIDs))
}

type MoveArgs struct {
	Shard int
	GID   int

	ClientId int
	OpId     int
}

func (m MoveArgs) String() string {
	return fmt.Sprintf("Move%v-%v %v->%v", m.ClientId, m.OpId, m.Shard, m.GID)
}

type QueryArgs struct {
	Num int // desired config number

	ClientId int
	OpId     int
}

func (q QueryArgs) String() string {
	return fmt.Sprintf("Query%v-%v %v", q.ClientId, q.OpId, q.Num)
}

type OpReply struct {
	Err    Err
	Config Config
}

func (o OpReply) String() string {
	s := fmt.Sprintf("%v", o.Err)
	if o.Config.Num != 0 {
		s += fmt.Sprintf(", config %v", o.Config.Shards)
	}
	return s
}

// HELPER FUNCTION

func groupsToGids(groups map[int][]string) []int {
	gids := make([]int, 0)
	for gid, _ := range groups {
		gids = append(gids, gid)
	}
	return gids
}
