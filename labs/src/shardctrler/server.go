package shardctrler

import (
	"6.824/debug"
	"6.824/raft"
	"sort"
	"time"
)
import "6.824/labrpc"
import "sync"
import "6.824/labgob"

// CONST

const (
	_NA = -1

	// log verbosity level
	vBasic debug.Verbosity = iota
	vVerbose
	vExcessive

	// log topic
	tTrace    debug.Topic = "TRCE"
	tError    debug.Topic = "ERRO"
	tSnapshot debug.Topic = "SNAP"

	tClient   debug.Topic = "CLNT"
	tKVServer debug.Topic = "KVSR"

	// timing
	_MinInterval       = 10 * time.Millisecond
	_HeartBeatInterval = 100 * time.Millisecond
)

// DATA TYPE

type ShardCtrler struct {
	mu      sync.Mutex
	me      int
	rf      *raft.Raft
	applyCh chan raft.ApplyMsg

	// Your data here.
	configs []Config // indexed by config num

	gidShardsMap map[int][]int

	lastOps      map[int]LastOp
	applyOpChans map[int]chan Op
}

type LastOp struct {
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

func (op *Op) equal(op1 *Op) bool {
	if op.ClientId == op1.ClientId && op.OpId == op1.OpId {
		return true
	}
	return false
}

// RPC

func (sc *ShardCtrler) Join(args *JoinArgs, reply *OpReply) {
	// Your code here.
	op := &Op{OpType: opJoin, Servers: args.Servers,
		ClientId: args.ClientId, OpId: args.OpId}
	sc.processOp(opJoin, op, reply)
}

func (sc *ShardCtrler) Leave(args *LeaveArgs, reply *OpReply) {
	// Your code here.
	op := &Op{OpType: opLeave, GIDs: args.GIDs,
		ClientId: args.ClientId, OpId: args.OpId}
	sc.processOp(opLeave, op, reply)
}

func (sc *ShardCtrler) Move(args *MoveArgs, reply *OpReply) {
	// Your code here.
	op := &Op{OpType: opMove, Shard: args.Shard, GID: args.GID,
		ClientId: args.ClientId, OpId: args.OpId}
	sc.processOp(opMove, op, reply)
}

func (sc *ShardCtrler) Query(args *QueryArgs, reply *OpReply) {
	// Your code here.
	op := &Op{OpType: opQuery, Num: args.Num,
		ClientId: args.ClientId, OpId: args.OpId}
	sc.processOp(opQuery, op, reply)
}

func (sc *ShardCtrler) processOp(join string, op *Op, reply *OpReply) {
	sc.mu.Lock()
	// If op is the same as lastOp
	if lastOp, ok := sc.lastOps[op.ClientId]; ok && op.equal(&lastOp.Op) {
		sc.mu.Unlock()
		reply.Err, reply.Config = OK, lastOp.QueryResult
		return
	}

	// Send op to raft by rf.Start()
	commandIndex, _, isLeader, currentLeader := sc.rf.StartWithCurrentLeader(*op)
	//		if not isLeader
	if !isLeader {
		sc.mu.Unlock()
		reply.Err, reply.CurrentLeader = ErrWrongLeader, currentLeader
		return
	}

	// make a channel for Op transfer, if not existing
	if _, ok := sc.applyOpChans[commandIndex]; !ok {
		sc.applyOpChans[commandIndex] = make(chan Op)
	}
	sc.mu.Unlock()

	// Wait for the op to be applied
	applied := sc.waitApply(commandIndex, op)

	switch applied {
	// TODO
	}
}

// FOR TESTER

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

// needed by shardkv tester
func (sc *ShardCtrler) Raft() *raft.Raft {
	return sc.rf
}

// APPLY_MSG

//
// wait for the commandIndex to be applied, return true if it's the same op
//
func (sc *ShardCtrler) waitApply(commandIndex int, op *Op) Err {
	sc.mu.Lock()
	ch := sc.applyOpChans[commandIndex]
	sc.mu.Unlock()
	// Wait for the commandIndex to be commited in raft and applied in kvserver
	select {
	case appliedOp := <-ch:
		if op.equal(&appliedOp) {
			// return OK if it's the same op
			return OK
		} else {
			// different op with the index applied
			return ErrNotApplied
		}
	case <-time.After(_HeartBeatInterval * 5):
		// waitApply timeout
		return ErrTimeout
	}
}

//
// iteratively receive ApplyMsg from raft and apply it to the kvserver, notify relevant RPC (leader server)
//
func (sc *ShardCtrler) receiveApplyMsg() {
	var applyMsg raft.ApplyMsg
	for {
		// Recieve ApplyMsg from raft
		applyMsg = <-sc.applyCh
		// if ApplyMsg is a command
		if applyMsg.CommandValid {
			sc.mu.Lock()
			// map command to Op
			op := applyMsg.Command.(Op)
			// command is not processed
			if lastOp, started := sc.lastOps[op.ClientId]; !started || !op.equal(&lastOp.Op) {
				// Apply the command to shard controller
				switch op.OpType {
				case opJoin:
					// if no group yet
					start := false
					if len(sc.configs[sc.now()].Groups) == 0 {
						start = true
					}
					// add new groups
					nextConfig := sc.makeNextConfig()
					for k, v := range op.Servers {
						nextConfig.Groups[k] = v
						sc.gidShardsMap[k] = []int{}
					}
					// if no group yet, add all shards to a group
					if start {
						gidMin, _ := sc.findMinMaxGid()
						sc.gidShardsMap[gidMin] = make([]int, NShards)
						for i := 0; i < NShards; i++ {
							sc.gidShardsMap[gidMin][i] = i
						}
					}
					// rebalance and add config
					sc.rebalance(&nextConfig.Shards)
					sc.configs = append(sc.configs, nextConfig)
				case opLeave:

				}
			} else {
				// TODO: log
			}
			ch, ok := sc.applyOpChans[applyMsg.CommandIndex]
			sc.mu.Unlock()

			// Send Op to RPC if relevant channel exists
			if ok {
				select {
				case ch <- op:
				case <-time.After(_MinInterval):
				}
			}
		}
	}
}

func (sc *ShardCtrler) joinAndRebalance(newGroups map[int][]string) {
	oldGroups := sc.configs[sc.now()].Groups

	sortedGidShards := makeSortedGidShards(sc.configs[sc.now()])

	sizeS, _, sizeL, numL := configStat(len(oldGroups) + len(newGroups))

	// move shards to new groups
	shards := sc.configs[sc.now()].Shards
	newGids := mapToSlice(newGroups)
	newGidInd := 0
	numLRemain := numL
	for _, gidShards := range sortedGidShards {
		var moveNum int
		groupSize := len(gidShards.shards)
		if groupSize > sizeS {
			if numLRemain > 0 {
				moveNum = groupSize - sizeL
				numLRemain--
			} else {
				moveNum = groupSize - sizeS
			}
			for i := 0; i < moveNum; i++ {
				shards[gidShards.shards[i]] = newGids[newGidInd]
				newGidInd++
				if newGidInd >= len(newGids) {
					newGidInd = 0
				}
			}
		}
	}

	// add new servers to groups
	groups := copyGroups(oldGroups)
	for k, v := range newGroups {
		groups[k] = v
	}

	// append new config
	num := sc.now()+1
	sc.configs = append(sc.configs, Config{num, shards, groups})
}

//
// calculate group size (number of shards), and how many groups are with the size
//
// return (size of an S group, number of S groups, size of an L group, number of L groups)
//
func configStat(configVol int) (int, int, int, int) {

}


func mapToSlice(servers map[int][]string) []int {

}

//
// make a map from gid to shards they handle,
// sorted by number of shards and gid (large to small)
//
// field shards is also sorted
//
func makeSortedGidShards(c Config) []gidShards {

}

type gidShards struct {
	gid int
	shards []int
}

//
// must be called with sc.mu.Lock()
//
func (sc *ShardCtrler) now() int {
	return len(sc.configs) - 1
}

//
// must be called with sc.mu.Lock()
//
func (sc *ShardCtrler) _rebalance() {
	currConfig, lastConfig := sc.configs[sc.now()], sc.configs[sc.now()-1]
	currConfigSize, lastConfigSize := len(currConfig.Groups), len(lastConfig.Groups)

	gidSizes := make(map[int]int, currConfigSize)

	oldGids := make([]int, lastConfigSize)
	i := 0
	for k, _ := range lastConfig.Groups {
		oldGids[0] = k
		i++
	}
	sort.Ints(oldGids)

	if currConfigSize > lastConfigSize {
		newGids := make([]int, currConfigSize-lastConfigSize)
		i := 0
		for k, _ := range currConfig.Groups {
			if _, ok := lastConfig.Groups[k]; !ok {
				newGids[i] = k
				i++
			}
		}
		sort.Ints(newGids)

		sort.Slice(newGids, less)

		smallGroupSize := NShards / currConfigSize
		largeGroupNum := NShards % currConfigSize
		largeGroupSize := smallGroupSize + 1
		smallGroupNum := currConfigSize - largeGroupNum

		for i, v := range lastConfig.Shards {
			gidSizes[v] += 1
			if gidSizes[v] <= smallGroupSize {
				currConfig.Shards[i] = v
				continue
			}
			if gidSizes[v] == largeGroupSize {
				if largeGroupNum > 0 {
					currConfig.Shards[i] = v
					largeGroupNum -= 1
					continue
				} else {

				}
			}
			if
		}
	}

	var currentGids, lastGids []int

	for k, _ := range lastConfig.Groups {
		lastGids = append(lastGids, k)
		sort.Ints(lastGids)
	}
	for k, _ := range currConfig.Groups {
		if _, ok := lastGids[k]; !ok {
			currentGids = append(currentGids, k)
		}
	}
}

func (sc *ShardCtrler) makeNextConfig() Config {
	// TODO
	return Config{}
}

func (sc *ShardCtrler) findMinMaxGid() (int, int) {
	// TODO
	return _NA, _NA
}

func (sc *ShardCtrler) rebalance(shards *[NShards]int) {
	// TODO
}

func copyGroups(groups map[int][]string) map[int][]string {
	newGroups := make(map[int][]string)
	for k, v := range groups {
		newGroups[k] = v
	}
	return newGroups
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
	sc.gidShardsMap = map[int][]int{}

	sc.lastOps = map[int]LastOp{}
	sc.applyOpChans = map[int]chan Op{}

	// Start receiving ApplyMsg from raft
	go sc.receiveApplyMsg()

	return sc
}
