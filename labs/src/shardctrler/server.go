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

type ShardCtrler struct {
	mu      sync.Mutex
	me      int
	rf      *raft.Raft
	applyCh chan raft.ApplyMsg

	// Your data here.
	configs []Config // indexed by config num

	gidShards map[int][]int

	lastOps     map[int]*_LastOp     // clientId -> _LastOp
	resultChans map[int]chan _Result // commandIndex -> chan _Result
}

type _LastOp struct {
	Op          Op
	QueryResult Config
}

type Op struct {
	// Your data here.
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

func (op *Op) isSame(op1 *Op) bool {
	if op.ClientId == op1.ClientId && op.OpId == op1.OpId {
		return true
	}
	return false
}

func (op *Op) format() string {
	var content string
	switch op.OpType {
	case opJoin:
		content = fmt.Sprint(groupsGids(op.Servers))
	case opLeave:
		content = fmt.Sprint(op.GIDs)
	case opMove:
		content = fmt.Sprintf("%v->%v", op.Shard, op.GID)
	case opQuery:
		content = fmt.Sprint(op.Num)
	}
	return fmt.Sprintf("%v%v-%v %v", op.OpType, op.ClientId, op.OpId, content)
}

//
// transfer op result among methods
//
type _Result struct {
	err    Err
	config Config
	opType string
	op     *Op
}

// RPC

func (sc *ShardCtrler) Join(args *JoinArgs, reply *OpReply) {
	// Your code here.
	op := &Op{OpType: opJoin, Servers: args.Servers,
		ClientId: args.ClientId, OpId: args.OpId}
	sc.processOp(op, reply)
}

func (sc *ShardCtrler) Leave(args *LeaveArgs, reply *OpReply) {
	// Your code here.
	op := &Op{OpType: opLeave, GIDs: args.GIDs,
		ClientId: args.ClientId, OpId: args.OpId}
	sc.processOp(op, reply)
}

func (sc *ShardCtrler) Move(args *MoveArgs, reply *OpReply) {
	// Your code here.
	op := &Op{OpType: opMove, Shard: args.Shard, GID: args.GID,
		ClientId: args.ClientId, OpId: args.OpId}
	sc.processOp(op, reply)
}

func (sc *ShardCtrler) Query(args *QueryArgs, reply *OpReply) {
	// Your code here.
	op := &Op{OpType: opQuery, Num: args.Num,
		ClientId: args.ClientId, OpId: args.OpId}
	sc.processOp(op, reply)
}

//
// process Op and set RPC reply
//
func (sc *ShardCtrler) processOp(op *Op, reply *OpReply) {
	sc.mu.Lock()
	// If op is the same as lastOp
	if lastOp, ok := sc.lastOps[op.ClientId]; ok && op.isSame(&lastOp.Op) {
		sc.mu.Unlock()
		LogCtrler(Verbose, Warn, sc.me, "%v already processed... (processOp)\n", op.format())
		reply.Err, reply.Config = OK, lastOp.QueryResult
		return
	}

	// Send op to raft by rf.Start()
	commandIndex, _, isLeader, currentLeader := sc.rf.StartWithCurrentLeader(*op)
	//		if not isLeader
	if !isLeader {
		sc.mu.Unlock()
		LogCtrler(Verbose, Warn, sc.me, "notLeader... (processOp)")
		reply.Err, reply.CurrentLeader = ErrWrongLeader, currentLeader
		return
	}

	// Make a channel for _Result transfer, if not existing
	if _, ok := sc.resultChans[commandIndex]; !ok {
		sc.resultChans[commandIndex] = make(chan _Result)
	}
	sc.mu.Unlock()

	// Wait for the op to be applied
	result := sc.waitApply(commandIndex, op)
	reply.Err, reply.Config = result.err, result.config
}

//
// wait for the commandIndex to be applied, return true if it's the same op
//
func (sc *ShardCtrler) waitApply(commandIndex int, op *Op) _Result {
	sc.mu.Lock()
	ch := sc.resultChans[commandIndex]
	sc.mu.Unlock()
	// Wait for the commandIndex to be commited in raft and applied in kvserver
	result := _Result{}
	select {
	case result = <-ch:
		if op.isSame(result.op) {
			switch result.err {
			case OK:
				LogCtrler(Basic, Ctrler, sc.me, "log%v: %v applied! (waitApply)", commandIndex, op.format())
			}
		} else {
			// different op with the index applied
			LogCtrler(Basic, Warn, sc.me, "other log%v applied... (%v /waitApply)\n", commandIndex, op.format())
			result.err = ErrNotApplied
		}
	case <-time.After(HeartBeatsInterval * 5):
		// waitApply timeout
		LogCtrler(Basic, Warn, sc.me, "log%v: %v timeout... (waitApply)\n", commandIndex, op.format())
		result.err = ErrTimeout
	}

	return result
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

// APPLY_MSG

//
// Iteratively receive ApplyMsg from raft and apply it to the kvserver,
// notify relevant RPC (leader server)
//
func (sc *ShardCtrler) receiveApplyMsg() {
	var applyMsg raft.ApplyMsg
	for {
		applyMsg = <-sc.applyCh
		// If ApplyMsg is a command
		if applyMsg.CommandValid {
			sc.mu.Lock()

			result := _Result{}
			op := applyMsg.Command.(Op)
			// If command is not processed
			if lastOp, started := sc.lastOps[op.ClientId]; !started || !op.isSame(&lastOp.Op) {
				result.err = OK
				// Apply the command to shard controller
				switch op.OpType {
				case opJoin:
					sc.applyJoin(&op)
				case opLeave:
					sc.applyLeave(&op)
				case opMove:
					sc.applyMove(&op)
				case opQuery:
					if op.Num != -1 && op.Num <= sc.now() {
						result.config = sc.configs[op.Num]
					} else {
						result.config = sc.configs[sc.now()]
					}
				}
				// Remember lastOp for each client
				sc.lastOps[op.ClientId] = &_LastOp{op, result.config}
				// log
				LogCtrler(Basic, Apply, sc.me, "%v applied!", op.format())
				// debug
				sc.logGidShards()
			} else {
				result.err = ErrDuplicate
				LogCtrler(Basic, Warn, sc.me, "%v already processed... (receiveApplyMsg)\n", op.format())
			}

			ch, ok := sc.resultChans[applyMsg.CommandIndex]
			sc.mu.Unlock()

			if ok {
				// Send Op to RPC if relevant channel exists
				result.op = &op
				select {
				case ch <- result:
				case <-time.After(MinInterval):
				}
			}
		}
	}
}

func (sc *ShardCtrler) applyJoin(op *Op) {
	// if no group yet
	start := false
	if len(sc.configs[sc.now()].Groups) == 0 {
		start = true
	}
	// add new groups
	newConfig := sc.makeNewConfig()
	for k, v := range op.Servers {
		newConfig.Groups[k] = v
		sc.gidShards[k] = []int{}
	}
	// if no group yet, add all shards to a group
	if start {
		gidMin, _ := sc.findMinMaxGid()
		sc.gidShards[gidMin] = make([]int, NShards)
		for i := 0; i < NShards; i++ {
			sc.gidShards[gidMin][i] = i
		}
	}
	// rebalance and add new config
	sc.rebalance(&newConfig.Shards)
	sc.configs = append(sc.configs, *newConfig)
}

func (sc *ShardCtrler) applyLeave(op *Op) {
	newConfig := sc.makeNewConfig()
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
		gidMin, _ := sc.findMinMaxGid()
		gids := make([]int, 0)
		for _, v := range removedGidShards {
			gids = append(gids, v...)
		}
		sort.Ints(gids)
		sc.gidShards[gidMin] = append(sc.gidShards[gidMin], gids...)
		// rebalance
		sc.rebalance(&newConfig.Shards)
	}
	// add new config
	sc.configs = append(sc.configs, *newConfig)
}

func (sc *ShardCtrler) applyMove(op *Op) {
	newConfig := sc.makeNewConfig()
	// find and remove shard from prev group
out:
	for gid, shards := range sc.gidShards {
		for ind, shard := range shards {
			if shard == op.Shard {
				sc.gidShards[gid] = append(shards[:ind], shards[ind+1:]...) // TODO: maybe problematic
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

// ShardCtrler Helpers, called with sc.mu.Lock()

//
// Make a new Config with current Shards and Groups
//
func (sc *ShardCtrler) makeNewConfig() *Config {
	config := Config{}
	config.Num = sc.now() + 1
	config.Groups = map[int][]string{}
	config.Shards = sc.configs[sc.now()].Shards
	for k, v := range sc.configs[sc.now()].Groups {
		config.Groups[k] = v
	}
	return &config
}

//
// Rebalance ShardCtrler.gidShards, apply the result to new Config.Shards
//
func (sc *ShardCtrler) rebalance(shardGids *[NShards]int) {
	// iteratively move a shard from gidMax to gidMin
	var shard int
	for {
		gidMin, gidMax := sc.findMinMaxGid()
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
func (sc *ShardCtrler) findMinMaxGid() (int, int) {
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
func (sc *ShardCtrler) now() int {
	return len(sc.configs) - 1
}

// DEBUG

//
// for debug only
//
// must be called with sc.mu.Lock()
//
func (sc *ShardCtrler) logGidShards() {
	config := sc.configs[sc.now()]
	LogCtrler(Stale, Trace, sc.me, "shards%v: %v (receiveApplyMsg)\n",
		config.Num, fmt.Sprint(config.Shards))
	LogCtrler(Stale, Trace, sc.me, "groups%v: %v (receiveApplyMsg)\n",
		config.Num, fmt.Sprint(groupsGids(config.Groups)))
	LogCtrler(Stale, Trace, sc.me, "gidShards%v: %v (receiveApplyMsg)\n",
		config.Num, fmt.Sprint(sc.gidShards))
	if len(config.Groups) != len(sc.gidShards) {
		log.Fatalln("Groups != gidShards!!!")
	}
}

// HELPER

func groupsGids(groups map[int][]string) []int {
	gids := make([]int, 0)
	for gid, _ := range groups {
		gids = append(gids, gid)
	}
	return gids
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

	sc.configs = make([]Config, 1)
	sc.configs[0].Groups = map[int][]string{}

	labgob.Register(Op{})
	sc.applyCh = make(chan raft.ApplyMsg)
	sc.rf = raft.Make(servers, me, persister, sc.applyCh)

	// Your code here.
	sc.gidShards = map[int][]int{}

	sc.lastOps = map[int]*_LastOp{}
	sc.resultChans = map[int]chan _Result{}

	// Start receiving ApplyMsg from raft
	go sc.receiveApplyMsg()

	return sc
}
