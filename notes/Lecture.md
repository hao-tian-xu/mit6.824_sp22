# L1. Introduction

### Introduction

- Reason
  - Parallelism
  - Fault tolerance
  - Physical
  - Security / Isolated
- Challenge
  - Concurrency
  - Partial Failure
  - Performance

- Course Structure
  - Lectures: big ideas, case study
  - Papers: what the basic problems are, what the ideas are
    - how to read a paper rapidly and skip over the parts that maybe aren't that important and sort of focus on teasing out the important ideas
  - Exams
  - Labs
  - Project (optional)



### Concepts

- Infrastructure for Applications
  - Storage
  - Communication
  - Computation

- Implementation
  - RPC, threads, concurrency control
- Performance
  - Scalability: 2 x computers -> 2 x throughput
- Fault tolerance
  - Availability
  - Recoverability
    - Non-volatile storage
    - Replication
- Consistency
  - `Put(k, v)`, `Get(k) -> v`: multiple copies



### MapReduce

- Job: Map tasks, Reduce tasks



# L2. Remote Procedure Call and Threads

### Go

- Why Go
  - Good support for threads
    - Garbage collection
- Tutorial: Effective Go



### Threads

- Go routine

- Why threads

  - I/O concurrency

  - Parallelism
  - Convenience



### Thread Challenges

- Race
  - Lock
  - or avoid sharing mutable data
- Coordination
  - Go channels and/or `sync.Cond` or `sync.WaitGroup`
- Deadlock
  - Cycles via locks and/or communication



### Web Crawler

- When to use sharing and locks, versus channels?
  - Most problems can be solved in either style
    - state -- sharing and locks
    - communication -- channels
  - For the 6.824 labs, I recommend 
    - sharing+locks for state,
    - and `sync.Cond` or channels or `time.Sleep()` for waiting/notification.



## Remote Procedure Call (RPC)

- Overview
  - a key piece of distributed system machinery; all the labs use RPC
  - goal: easy-to-program client/server communication
  - hide details of network protocols
  - convert data (strings, arrays, maps, &c) to "wire format"
  - portability / interoperability
- RPC message diagram
  - Client -> request -> Server
  - Server -> response -> Client

### Software structure

```
  client app        handler fns
   stub fns         dispatcher
   RPC lib           RPC lib
     net  ------------ net
```

### Go example: `kv.go` on schedule page

- A toy key/value storage server -- `Put(key,value),` `Get(key)`->value
- Uses Go's RPC library
- Common:
  - Declare Args and Reply struct for each server handler.
- Client:
  - `connect()`'s `Dial()` creates a TCP connection to the server
  - `get()` and `put()` are **client "stubs"**
  - `Call()` asks the RPC library to perform the call
    - you specify server function name, arguments, place to put reply
    - library marshalls args, sends request, waits, unmarshalls reply
    - return value from `Call()` indicates whether it got a reply
    - usually you'll also have a `reply.Err` indicating service-level failure
- Server:
  - Go requires server **to declare an object with methods as RPC handlers**
  - Server then registers that object with the RPC library
  - Server accepts TCP connections, gives them to RPC library
  - The RPC library
    - reads each request
    - creates a new goroutine for this request
    - unmarshalls request
    - looks up the named object (in table create by Register())
    - calls the object's named method (dispatch)
    - marshalls reply
    - writes reply on TCP connection
  - The server's Get() and Put() handlers
    - Must lock, since RPC library creates a new goroutine for each request
    - read args; modify reply

### A few details

- What does a failure look like to the client RPC library?

  - Simplest failure-handling scheme: "best-effort RPC"

    - scheme
      - Call() waits for response for a while
      - If none arrives, re-send the request
      - Do this a few times
      - Then give up and return an error
    - is best effort ever OK?
      - read-only operations
      - operations that do nothing if repeated
        - e.g. DB checks if record has already been inserted

  - Better RPC behavior: "at-most-once RPC"

    - idea: client re-sends if no answer;

      - server RPC code detects duplicate requests,
      - returns previous reply instead of re-running handler

    - Q: how to detect a duplicate request?

      - client includes unique ID (XID) with each request
        - uses same XID for re-send

    - server:

      - ```go
        if seen[xid]:
        	r = old[xid]
        else
        	r = handler()
        	old[xid] = r
        	seen[xid] = true
        ```

    - some at-most-once complexities

      - *<u>see notes</u>*

    - Go RPC is a simple form of "at-most-once"


### *Thinking*

- 简要介绍了Go的一些并行计算的特征
- 通过Web Crawler对比了state lock模型和channel模型的区别
  - 如果是状态的改变使用lock
  - 如果是等待和通知使用channel
- 笔记中详细介绍了RPC，是分布式系统的一个关键机制，
  - 易于编程的客户/服务器通信、隐藏网络协议的细节、将数据（字符串、数组、map等）转换成 "wire format"
  - Go本身有实现RPC的package

# L3. The Google File System

### Why Hard

- Performance -> Sharding
- Many servers -> Fault
- -> Replication
- -> Inconsistencies
- Conssitency -> Low performance

## GFS

- Big, Fast (Paralell Access)
- Global
- Sharding (High Aggregate Speed)
- Automatic recovery
- Single data center
- Internal use
- Big sequential access

### Design

- Client
- Master
- Chunkserver

### Master Data

- file name -> 
  - array of chunk handles (non-volatile)
- chunk handles -> 
  - list of chunk servers
  - version # (non-volatile)
  - primary
  - lease expiration
- log, checkpoint -> 
  - disk
    - log is faster than b-tree in disk
    - after a master fail, new master only read log from when the checkpoint was created

### Read

1. client -> name, offset -> master
2. master -> handles, list of severs -> client (cached)
3. client -> request -> chunkserver
4. chunkserver -> data -> client

### Write (Record Append)

- on master
  - if no primary
    - find up-to-date replicas
    - pick primary
    - increment version #
    - version # -> primary and secondaries
    - lease -> primary
- primary picks offset
- all replicas told to write at offset
- if all "yes", "success" -> client
  else, "no" -> client



# L4. Primary-Backup Replication

### Failures

- yes: fail-stop failures
- no: logic bugs, configuration errors, malicious
- maybe: earthquake...

### Challenges

- has primary failed?
  - split-brain system
- keep primary and backup in sync
  - apply changes in order
  - deal with non-determinism
- fail over

### Two Approaches

1. State transfer
   - disadvantage: if a operation generates many states, it's expensive to send many data
2. Replicate state machine (RSM)

### Level of operations to replicate

- application-level operations (file append, write)
- machine level
  - transparent!
  - virtual machines

## VMFT: exploit virtualization

- transparent replication
- appears to client that server is a single machine
- VMware product

### Overview

- killover

### Goal: behave like a single machine

- divergence source
  - non-deterministic instruction
  - input packets
  - timer interrupts
  - multicore -> disallow

### Interrupts

- ++*<u>question</u>*: deterministic instructions are not sent throught the logging channel

### Output Rule

- backup acknowledgment



# L5. Fault Tolerance: Raft (1)

### Pattern: single point of failure

- MR replicates computation but relies on a single master to organize
* GFS replicates data but relies on the master to pick primaries
- VMware FT replicates service but relies on test-and-set to pick primary
- -> all rely on a single entity to make critical decisions to avoid split brain

### Idea: test-and-set server replication

- problem: split brain

### Network partition: majority rule

- overlap in majorities

### Protocols using Quorum

- around 1990
  - Paxos
  - View-stamped replication
- Raft (2014)

## Raft Overview

### Election

- split vote
  - random timeout
- votes are written in disk (one vote per term -> one leader per term)

### Logs may diverge

### Lab

- all things in figure 2 should be implemented



# L7. Fault Tolerance: Raft (2)

### Divergence (cont.)

### Log Catchup

subtlety in commit rule

### Service recovery

### Using Raft

### Correctness: Linearizability

- total order of operations
- match real-time
- read return results of the last write



# L9. Zookeeper Case Study

Main Purpose: configuration / coordination

### Throughput

- asynchronous writes --> single write to disk with multiple `Put()`s
- read is not linearizable --> `Get()`s from different servers

###  ZK guarantees

- Linearizable Writes
- FIFO client order
  - Writes - client-specified order (in  overall asynchronous order)
  - Reads

### Motivation

- Test-and-Set
- Configuration Info
- Master Election

### API

- znodes

  - regular
  - ephemeral
  - sequential

- RPCs

  - create(path, data, flags)
    - exclusive -- only first create indicates success
  - delete(path, version)
    - if znode.version = version, then delete
  - exists(path, watch)
    - watch=true means also send notification if path is later created/deleted
  - getData(path, watch)
  - setData(path, data, version)
    - if znode.version = version, then update
  - getChildren(path, watch)
  - sync()
    - sync then read ensures writes before sync are visible to same client's read
    - client could instead submit a write

- Example: add one to a number stored in a ZooKeeper znode ("mini-transaction")

  - ```pseudocode
    while true:
        x, v := getData("f")
        if setData(x + 1, version=v):
          break
    ```

- Herd effects

  - N^2^ complexity
  - Example: Locks without Herd Effect ("scalable lock")
      (look at pseudo-code in paper, Section 2.4, page 6)
      1. create a "sequential" file
      2. list files
      3. if no lower-numbered, lock is acquired!
        4. if exists(next-lower-numbered, watch=true)
             1. wait for event...
        5. goto 2



# L10: Chain Replication

### Approaches to RSMs

1. Run all ops through Raft/Paxos (Lab3)
2. Configuration service + Primary/Backup replication (Common)

### Chain replication

- Purpose: P/B replication for approach 2
- Pros
  - Reads ops involve 1 server
  - Simple recovery plan
  - Linearizability
  - Influential

### Overview

- Servers in a chain: Head and Tail

### CR properties (wrt. Raft)

- Pros:
  - Client RPCs split between head and tail
  - Head sends update once
  - Read ops involve only tail
  - Simple crash recovery
- Cons:
  - One failure requires reconfiguration
  - Slow server in Chain Replication is very damaging

### Extension for read parallelism

- Splits object across many chains
  - CH1: S1 S2 S3
  - CH2: S2 S3 S1
  - CH3: S3 S1 S2
- --> scale + linearizability



# L11. Distributed Transactions

- 2-phase locking (2PL)
- 2-phase commit (2PC)

### Problem: cross-machine atomic ops

- goal: atomicity w.r.t. failures + concurrency

### Transactions

- ACID: Atomic, Consistent, Isolated, Durable
- out focus: fistributed transactions

### Isolation: Serializability

- Problem cases
  - t1 <- get(x); transfer(x, y); t2 <- get(y)
  - put(x); get(x); get(y); put(y)

### Concurrency control

1. pessimistic (locks) (today)
2. optimistic (no locks, abort if not serializable)

### Two-phase locking (lock per record)

- one way to implement serializability (single machine)
- 2PL rules
  - T only acquires lock before using
  - T holds ultil commit / abort
- Two-phase locking can produce deadlock
  - The system must detect (cycles? timeout?) and abort a transaction

### Two-phase commit: Crashes

- We want "atomic commit" (multiple machines)
- Discussion
  - Use raft to make Coordinator available
  - Raft ~ 2PC?
    - Raft: all servers do the same (RSM)
    - 2PC: all servers operate on different data
      atomic ops across servers



# L12. Frangipani

- Network file systems
- Focus (a gentle introduction)
  - cache coherence
  - distributed locking
  - distributed crash recovery

### Traditional Network FS vs Frangipani

- simple client and complex FS
- Frangipani: complex client with FS

### Use case

- Researchers: compiling, editing
- Potentially share files/dr
  - user-to-user sharing
  - same user logs inot more than 1 workstations

### Design choices

- Caching (write-back)
- Strong consistency
- Performance

### Cache coherence / consistency

- lock server
  - busy, idle

### Atomicity

- same lock server

### Crsh recovery

- write-ahead logging



# L14. Spanner

- Wide-area transactions
  - R/W transaction: 2PC + 2PL + Paxos groups
  - R/O transaction: snapshot isolation + syncronized clocks
- Widely-used

### Organization

- different data centers
- multiple shards --> parallelism
- Paxos group per shard
  - --> data center: fault tolerance, slowness
- Replica close to clients

### Challenges

- Read of local replica yields latest Write
- Transactions across Shards
- Transactions must be serializable

### Read/Write transactions



## Read-only transactions

- 10 x Faster than R/W
  - Read from local shards
  - No lock
  - No 2PC

### Correctness

- Serializable
- External consistency ~ Linearizability (transaction version)

### Snapshot Isolation

- Timestamp

### Stale Replica

- Solution: "safte time"
  - Paxos sends writes in timestamp order
  - Before Rx@15, wait for Wx>@15
    (Also wait for transactions that have prepared but not committed)

### Clocks must be perfect

- Matters only for R/O transactions
- Difficulty: clock is naturally drifty
  - --> atomic clocks --> synchronize clocks (gps) --> $\epsilon$ few microsec to few millisec
- Solution: time stamps are intervals
  - [earliest, latest]
- Rule change
  - Start rule: `timestamp = now.latest`
  - Commit wait: delay the commit until `timestamp < now.earlist`

### Discussion

- R/W transaction: 2PC + 2PL
- R/O transaction: 
  - snapshot isolation 
  - --> external consistency
  - --> time stamp order
  - --> time intervals







