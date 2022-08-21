package shardctrler

import (
	"6.824/raft"
	. "6.824/util"
	"fmt"
	"log"
	"sort"
	"time"
)
import "6.824/labrpc"
import "sync"
import "6.824/labgob"

// DATA TYPE

type Op struct {
	OpType string
	// Join
	Servers map[int][]string
	// Leave
	GIDs []int
	// Move
	Shard int
	GID   int
	// Query
	Num int

	ClientId int
	OpId     int
}

func (op *Op) String() string {
	var content string
	switch op.OpType {
	case opJoin:
		content = fmt.Sprint(groupsToGids(op.Servers))
	case opLeave:
		content = fmt.Sprint(op.GIDs)
	case opMove:
		content = fmt.Sprintf("%v->%v", op.Shard, op.GID)
	case opQuery:
		content = fmt.Sprint(op.Num)
	}
	return fmt.Sprintf("%v%v-%v %v", op.OpType, op.ClientId, op.OpId, content)
}

func (op *Op) isSame(op1 *Op) bool {
	if op.ClientId == op1.ClientId && op.OpId == op1.OpId {
		return true
	}
	return false
}

type _LastOp struct {
	op          Op
	queryResult Config
}

//
// transfer op result among methods
//
type _Result struct {
	err    Err
	config Config
	op     *Op
}

func (r _Result) String() string {
	return fmt.Sprintf("%v %v", r.err, r.op)
}

// SHARD CONTROLLER

type ShardCtrler struct {
	mu      sync.Mutex
	me      int
	rf      *raft.Raft
	applyCh chan raft.ApplyMsg

	configs   []Config      // indexed by config num
	gidShards map[int][]int // data structure to generate configs

	lastOps     map[int]*_LastOp     // clientId -> _LastOp
	resultChans map[int]chan _Result // commandIndex -> chan _Result
}

// RPC HANDLER

func (sc *ShardCtrler) Join(args *JoinArgs, reply *OpReply) {
	sc.lock("Join")
	defer sc.unlock("Join")

	op := &Op{OpType: opJoin, Servers: args.Servers,
		ClientId: args.ClientId, OpId: args.OpId}
	sc.processOpL(op, reply)
}

func (sc *ShardCtrler) Leave(args *LeaveArgs, reply *OpReply) {
	sc.lock("Leave")
	defer sc.unlock("Leave")

	op := &Op{OpType: opLeave, GIDs: args.GIDs,
		ClientId: args.ClientId, OpId: args.OpId}
	sc.processOpL(op, reply)
}

func (sc *ShardCtrler) Move(args *MoveArgs, reply *OpReply) {
	sc.lock("Move")
	defer sc.unlock("Move")

	op := &Op{OpType: opMove, Shard: args.Shard, GID: args.GID,
		ClientId: args.ClientId, OpId: args.OpId}
	sc.processOpL(op, reply)
}

func (sc *ShardCtrler) Query(args *QueryArgs, reply *OpReply) {
	sc.lock("Query")
	defer sc.unlock("Query")

	op := &Op{OpType: opQuery, Num: args.Num,
		ClientId: args.ClientId, OpId: args.OpId}
	sc.processOpL(op, reply)
}

//
// process Op and set RPC reply
//
func (sc *ShardCtrler) processOpL(op *Op, reply *OpReply) {
	sc.log(VBasic, TCtrler1, "process op %v, lastOp %v (processOpL)", op, sc.lastOps[op.ClientId])

	// If op is the same as lastOp
	if lastOp, ok := sc.lastOps[op.ClientId]; ok && op.isSame(&lastOp.op) {
		reply.Err, reply.Config = OK, lastOp.queryResult
		return
	}

	// Send op to raft by rf.Start()
	commandIndex, _, isLeader := sc.rf.Start(*op)
	//	if not leader
	if !isLeader {
		reply.Err = ErrWrongLeader
		return
	}

	// Wait for the op to be applied
	result := sc.waitApplyL(commandIndex, op)

	reply.Err = result.err
	reply.Config = result.config
}

//
// wait for the commandIndex to be applied, return true if it's the same op
//
func (sc *ShardCtrler) waitApplyL(commandIndex int, op *Op) _Result {
	// Make a channel for _Result transfer, if not existing
	if _, ok := sc.resultChans[commandIndex]; !ok {
		sc.resultChans[commandIndex] = make(chan _Result)
	}

	ch := sc.resultChans[commandIndex]
	result := _Result{}

	// Wait for the commandIndex to be commited in raft and applied in kvserver
	sc.unlock("waitApply")
	select {
	case result = <-ch:
	case <-time.After(MaxTimeout * HeartbeatsInterval):
		result.err = ErrTimeout
	}
	sc.lock("waitApply")

	delete(sc.resultChans, commandIndex)

	sc.log(VBasic, TCtrler1, "command %v apply result: %v (waitApplyL)", commandIndex, result)

	// Process result
	if result.err == OK && !op.isSame(result.op) {
		result.err = ErrNotApplied
	}

	return result
}

// APPLIER

//
// Iteratively receive ApplyMsg from raft and apply it to the kvserver,
// notify relevant RPC (leader server)
//
func (sc *ShardCtrler) receiveApplyMsg() {
	sc.lock("receiveApplyMsg")
	defer sc.unlock("receiveApplyMsg")

	for {
		sc.unlock("receiveApplyMsg")
		applyMsg := <-sc.applyCh
		sc.lock("receiveApplyMsg")

		// If ApplyMsg is a command
		if applyMsg.CommandValid {
			op := applyMsg.Command.(Op)

			result := sc.applyOpL(&op)

			if ch, ok := sc.resultChans[applyMsg.CommandIndex]; ok {
				result.op = &op

				sc.unlock("receiveApplyMsg")
				ch <- result
				sc.lock("receiveApplyMsg")
			}
		}
	}
}

func (sc *ShardCtrler) applyOpL(op *Op) _Result {
	result := _Result{}
	// If command is not processed
	if lastOp, started := sc.lastOps[op.ClientId]; !started || !op.isSame(&lastOp.op) {
		sc.log(VBasic, TCtrler2, "%v applied! (receiveApplyMsg)", op)

		result.err = OK

		// Apply the command to shard controller
		switch op.OpType {
		case opJoin:
			sc.applyJoinL(op)
		case opLeave:
			sc.applyLeaveL(op)
		case opMove:
			sc.applyMoveL(op)
		case opQuery:
			if op.Num != -1 && op.Num <= sc.nowL() {
				result.config = sc.configs[op.Num]
			} else {
				result.config = sc.configs[sc.nowL()]
			}
		}
		// Remember lastOp for each client
		sc.lastOps[op.ClientId] = &_LastOp{*op, result.config}

		// debug
		sc.logGidShardsL()

	} else {
		sc.log(VBasic, TWarn, "%v just processed... (receiveApplyMsg)", op)

		result.err = ErrDuplicate
	}
	return result
}

func (sc *ShardCtrler) applyJoinL(op *Op) {
	// if no group yet
	start := false
	if len(sc.configs[sc.nowL()].Groups) == 0 {
		start = true
	}
	// add new groups
	newConfig := sc.makeNewConfigL()
	for k, v := range op.Servers {
		newConfig.Groups[k] = v
		sc.gidShards[k] = []int{}
	}
	// if no group yet, add all shards to a group
	if start {
		gidMin, _ := sc.findMinMaxGidL()
		sc.gidShards[gidMin] = make([]int, NShards)
		for i := 0; i < NShards; i++ {
			sc.gidShards[gidMin][i] = i
		}
	}
	// rebalance and add new config
	sc.rebalanceL(&newConfig.Shards)
	sc.configs = append(sc.configs, *newConfig)
}

func (sc *ShardCtrler) applyLeaveL(op *Op) {
	newConfig := sc.makeNewConfigL()
	// remove groups, remember their shards
	removedGidShards := map[int][]int{}
	for _, gid := range op.GIDs {
		delete(newConfig.Groups, gid)
		removedGidShards[gid] = sc.gidShards[gid]
		delete(sc.gidShards, gid)
	}
	if len(newConfig.Groups) == 0 {
		// All groups left
		for i := 0; i < NShards; i++ {
			newConfig.Shards[i] = 0
		}
	} else {
		// add shards of removed groups to a remaing group,
		//  should be deterministic
		gidMin, _ := sc.findMinMaxGidL()
		gids := make([]int, 0)
		for _, v := range removedGidShards {
			gids = append(gids, v...)
		}
		sort.Ints(gids)
		sc.gidShards[gidMin] = append(sc.gidShards[gidMin], gids...)
		// rebalance
		sc.rebalanceL(&newConfig.Shards)
	}
	// add new config
	sc.configs = append(sc.configs, *newConfig)
}

func (sc *ShardCtrler) applyMoveL(op *Op) {
	newConfig := sc.makeNewConfigL()
	// find and remove shard from prev group
out:
	for gid, shards := range sc.gidShards {
		for ind, shard := range shards {
			if shard == op.Shard {
				sc.gidShards[gid] = append(shards[:ind], shards[ind+1:]...)
				break out
			}
		}
	}
	// add shard to new group
	sc.gidShards[op.GID] = append(sc.gidShards[op.GID], op.Shard)
	newConfig.Shards[op.Shard] = op.GID
	// add new config
	sc.configs = append(sc.configs, *newConfig)
}

// HELPER

//
// Make a new Config with current Shards and Groups
//
func (sc *ShardCtrler) makeNewConfigL() *Config {
	config := Config{}
	config.Num = sc.nowL() + 1
	config.Groups = map[int][]string{}
	config.Shards = sc.configs[sc.nowL()].Shards
	for k, v := range sc.configs[sc.nowL()].Groups {
		config.Groups[k] = v
	}
	return &config
}

//
// Rebalance ShardCtrler.gidShards, apply the result to new Config.Shards
//
func (sc *ShardCtrler) rebalanceL(shardGids *[NShards]int) {
	// iteratively move a shard from gidMax to gidMin
	var shard int
	for {
		gidMin, gidMax := sc.findMinMaxGidL()
		minGidShards, maxGidShards := sc.gidShards[gidMin], sc.gidShards[gidMax]
		if len(maxGidShards)-len(minGidShards) <= 1 {
			break
		}
		lastInd := len(maxGidShards) - 1
		shard, sc.gidShards[gidMax] = maxGidShards[lastInd], maxGidShards[0:lastInd]
		sc.gidShards[gidMin] = append(minGidShards, shard)
	}
	// apply the result to shardGids
	for gid, shards := range sc.gidShards {
		for _, shard := range shards {
			shardGids[shard] = gid
		}
	}
}

//
// In ShardCtrler.gidShards, find gid with minimal number of shards
// and maximal number of shards. The function is deterministic.
//
// return (gidMin, gidMax)
//
func (sc *ShardCtrler) findMinMaxGidL() (int, int) {
	gidMin, gidMax := NA, NA
	sizeMin, sizeMax := NShards+1, -1
	for gid, shards := range sc.gidShards {
		if len(shards) < sizeMin {
			sizeMin = len(shards)
			gidMin = gid
		} else if len(shards) == sizeMin && gid < gidMin {
			// deterministic
			gidMin = gid
		}
		if len(shards) > sizeMax {
			sizeMax = len(shards)
			gidMax = gid
		} else if len(shards) == sizeMax && gid > gidMax {
			// deterministic
			gidMax = gid
		}
	}
	return gidMin, gidMax
}

//
// return current Config index
//
func (sc *ShardCtrler) nowL() int {
	return len(sc.configs) - 1
}

// debug

func (sc *ShardCtrler) log(verbose LogVerbosity, topic LogTopic, format string, a ...interface{}) {
	LogCtrler(verbose, topic, sc.me, format, a...)
}

//
// for debug only
//
func (sc *ShardCtrler) logGidShardsL() {
	config := sc.configs[sc.nowL()]
	sc.log(VStale, TTrace, "shards%v: %v (receiveApplyMsg)\n",
		config.Num, fmt.Sprint(config.Shards))
	sc.log(VStale, TTrace, "groups%v: %v (receiveApplyMsg)\n",
		config.Num, fmt.Sprint(groupsToGids(config.Groups)))
	sc.log(VStale, TTrace, "gidShards%v: %v (receiveApplyMsg)\n",
		config.Num, fmt.Sprint(sc.gidShards))
	if len(config.Groups) != len(sc.gidShards) {
		log.Fatalln("Groups != gidShards!!!")
	}
}

func (sc *ShardCtrler) lock(method string) {
	sc.log(VExcessive, TTrace, "acquire lock (%v)", method)
	sc.mu.Lock()
}

func (sc *ShardCtrler) unlock(method string) {
	sc.log(VExcessive, TTrace, "release lock (%v)", method)
	sc.mu.Unlock()
}

// TESTER ONLY

//
// the tester calls Kill() when a ShardCtrler instance won't
// be needed again. you are not required to do anything
// in Kill(), but it might be convenient to (for example)
// turn off debug output from this instance.
//
func (sc *ShardCtrler) Kill() {
	sc.rf.Kill()
	// Your code here, if desired.
}

//
// needed by shardkv tester
//
func (sc *ShardCtrler) Raft() *raft.Raft {
	return sc.rf
}

// START

//
// servers[] contains the ports of the set of
// servers that will cooperate via Raft to
// form the fault-tolerant shardctrler service.
// me is the index of the current server in servers[].
//
func StartServer(servers []*labrpc.ClientEnd, me int, persister *raft.Persister) *ShardCtrler {
	sc := new(ShardCtrler)
	sc.me = me

	sc.log(VBasic, TCtrler2, "start controller")

	sc.configs = make([]Config, 1)
	sc.configs[0].Groups = map[int][]string{}

	labgob.Register(Op{})
	sc.applyCh = make(chan raft.ApplyMsg)
	sc.rf = raft.Make(servers, me, persister, sc.applyCh)

	sc.gidShards = map[int][]int{}

	sc.lastOps = map[int]*_LastOp{}
	sc.resultChans = map[int]chan _Result{}

	// Start receiving ApplyMsg from raft
	go sc.receiveApplyMsg()

	return sc
}
