# Lab 1: MapReduce

### Getting started

- [ ] a simple sequential mapreduce implementation in `src/main/mrsequential.go`
- [ ] a couple of MapReduce applications: word-count in `mrapps/wc.go`, and a text indexer in `mrapps/indexer.go`

### Your Job

- [ ] a distributed MapReduce, consisting of two programs, the coordinator and the worker.

### A few rules

- [ ] The map phase should **divide the intermediate keys into buckets** for `nReduce` reduce tasks, where `nReduce` is the number of reduce tasks -- argument that `main/mrcoordinator.go` passes to `MakeCoordinator()`. So, each mapper needs to create `nReduce` intermediate files for consumption by the reduce tasks.
- [ ] The worker implementation should **put the output of the X'th reduce task in the file** `mr-out-X`.
- [ ] A `mr-out-X` file should **contain one line per Reduce function output**. The line should be generated with the Go `"%v %v"` format, called with the key and value. Have a look in `main/mrsequential.go` for the line commented "this is the correct format". The test script will fail if your implementation deviates too much from this format.
- [ ] You can modify `mr/worker.go`, `mr/coordinator.go`, and `mr/rpc.go`. You can temporarily modify other files for testing, but make sure your code works with the original versions; we'll test with the original versions.
- [ ] The worker should **put intermediate Map output in files in the current directory**, where your worker can later read them as input to Reduce tasks.
- [ ] `main/mrcoordinator.go` expects `mr/coordinator.go` to **implement a `Done()` method** that returns true when the MapReduce job is completely finished; at that point, `mrcoordinator.go` will exit.
- [ ] **When the job is completely finished, the worker processes should exit.** A simple way to implement this is to use the return value from `call()`: if the worker fails to contact the coordinator, it can assume that the coordinator has exited because the job is done, and so the worker can terminate too. Depending on your design, you might also find it helpful to have a "please exit" pseudo-task that the coordinator can give to workers.

### Hints

- [ ] **The [Guidance page](https://pdos.csail.mit.edu/6.824/labs/guidance.html) has some tips on developing and debugging.**

- [ ] **One way to get started** is to modify `mr/worker.go`'s `Worker()` to send an RPC to the coordinator asking for a task. Then modify the coordinator to respond with the file name of an as-yet-unstarted map task. Then modify the worker to read that file and call the application Map function, as in `mrsequential.go`.

- [ ] The application Map and Reduce functions are loaded at run-time using the Go plugin package, from files whose names end in `.so`.

- [ ] If you change anything in the `mr/` directory, **you will probably have to re-build any MapReduce plugins you use**, with something like `go build -race -buildmode=plugin ../mrapps/wc.go`

- [ ] **This lab relies on the workers sharing a file system**. That's straightforward when all workers run on the same machine, but would require a global filesystem like GFS if the workers ran on different machines.

- [ ] A reasonable **naming convention for intermediate files** is `mr-X-Y`, where X is the Map task number, and Y is the reduce task number.

- [ ] The worker's map task code will need **a way to store intermediate key/value pairs in files** in a way that can be correctly read back during reduce tasks. One possibility is to use Go's `encoding/json` package. To write key/value pairs in JSON format to an open file:

  ```
    enc := json.NewEncoder(file)
    for _, kv := ... {
      err := enc.Encode(&kv)
  ```

  and to read such a file back:

  ```
    dec := json.NewDecoder(file)
    for {
      var kv KeyValue
      if err := dec.Decode(&kv); err != nil {
        break
      }
      kva = append(kva, kv)
    }
  ```

- [ ] The map part of your worker can **use the `ihash(key)` function (in `worker.go`) to pick the reduce task for a given key**.

- [ ] You can **steal some code from `mrsequential.go`** for reading Map input files, for sorting intermedate key/value pairs between the Map and Reduce, and for storing Reduce output in files.

- [ ] **The coordinator, as an RPC server, will be concurrent**; don't forget to lock shared data.

- [ ] **Use Go's race detector**, with `go build -race` and `go run -race`. `test-mr.sh` by default runs the tests with the race detector.

- [ ] **Workers will sometimes need to wait, e.g. reduces can't start until the last map has finished.** One possibility is for workers to periodically ask the coordinator for work, sleeping with `time.Sleep()` between each request. Another possibility is for the relevant RPC handler in the coordinator to have a loop that waits, either with `time.Sleep()` or `sync.Cond`. Go runs the handler for each RPC in its own thread, so the fact that one handler is waiting won't prevent the coordinator from processing other RPCs.

- [ ] The coordinator can't reliably distinguish between crashed workers, workers that are alive but have stalled for some reason, and workers that are executing but too slowly to be useful. **The best you can do is have the coordinator wait for some amount of time, and then give up and re-issue the task to a different worker.** For this lab, have the coordinator wait for ten seconds; after that the coordinator should assume the worker has died (of course, it might not have).

- [ ] **If you choose to implement Backup Tasks** (Section 3.6), note that we test that your code doesn't schedule extraneous tasks when workers execute tasks without crashing. Backup tasks should only be scheduled after some relatively long period of time (e.g., 10s).

- [ ] **To test crash recovery**, you can use the `mrapps/crash.go` application plugin. It randomly exits in the Map and Reduce functions.

- [ ] **To ensure that nobody observes partially written files in the presence of crashes**, the MapReduce paper mentions the trick of using a temporary file and atomically renaming it once it is completely written. You can use `ioutil.TempFile` to create a temporary file and `os.Rename` to atomically rename it.

- [ ] `test-mr.sh` runs all its processes in the sub-directory `mr-tmp`, so if something goes wrong and you want to look at intermediate or output files, look there. You can temporarily modify `test-mr.sh` to `exit` after the failing test, so the script does not continue testing (and overwrite the output files).

- [ ] `test-mr-many.sh` provides a bare-bones script for running `test-mr.sh` with a timeout (which is how we'll test your code). It takes as an argument the number of times to run the tests. You should not run several `test-mr.sh` instances in parallel because the coordinator will reuse the same socket, causing conflicts.

- [ ] **Go RPC sends only struct fields whose names start with capital letters. Sub-structures must also have capitalized field names.**

- [ ] When passing a pointer to a reply struct to the RPC system, **the object that `*reply` points to should be zero-allocated**. The code for RPC calls should always look like

  ```
    reply := SomeType{}
    call(..., &reply)
  ```

  **without setting any fields of reply before the call**. If you don't follow this requirement, there will be a problem when you pre-initialize a reply field to the non-default value for that datatype, and the server on which the RPC executes sets that reply field to the default value; you will observe that the write doesn't appear to take effect, and that on the caller side, the non-default value remains.

### *Thinking*

- 主要练习了**<u>RPC</u>**（Remote Procedure Call）的使用
- **<u>确定函数框架的方法</u>**：
  - 首先将主要函数的功能完整实现，复杂部分用子函数占位，并明确输入和输出
  - 将各函数树状深化
- **<u>指针</u>**：
  - 子函数修改参数数据时，需要传入指针
  - 将`array`的子项（可变数据类型）给变量赋值，并在之后的过程中进行修改，需要将指针赋值给变量，而不是子项本身
  - go中目前已知的本身为指针的数据类型只有`map`



# Lab 2: Raft



## Part 2A: leader election

- [ ] Implement Raft **leader election** and **heartbeats** (`AppendEntries` RPCs with no log entries). 
- [ ] The goal for Part 2A is 
  - [ ] for **a single** leader to be elected, 
  - [ ] for the leader **to remain the leader** if there are no failures, and 
  - [ ] for **a new leader to take over** if the old leader fails or if packets to/from the old leader are lost. 
- [ ] Run `go test -run 2A `to test your 2A code.

### Hint

- [ ] You can't easily run your Raft implementation directly; instead you should run it by way of the tester, i.e. `go test -run 2A `.
- [ ] **Follow the paper's Figure 2**. At this point you care about sending and receiving RequestVote RPCs, the Rules for Servers that relate to elections, and the State related to leader election,
- [x] Add the **Figure 2 state for leader election to the `Raft` struct** in `raft.go`. 
  - [x] You'll also need to define **a struct to hold information about each log entry**.
- [x] **Fill in the `RequestVoteArgs` and `RequestVoteReply` structs**. 
  - [x] Modify `Make()` to create a background goroutine that will kick off leader election periodically by sending out `RequestVote` RPCs when it hasn't heard from another peer for a while. 
  - [x] This way a peer will learn who is the leader, if there is already a leader, or become the leader itself. 
  - [x] Implement the `RequestVote()` RPC handler so that servers will vote for one another.
- [x] **To implement heartbeats, define an `AppendEntries` RPC struct** (though you may not need all the arguments yet), and 
  - [x] have the leader **send them out periodically**. 
  - [x] **Write an `AppendEntries` RPC handler method** that resets the election timeout so that other servers don't step forward as leaders when one has already been elected.
- [x] Make sure **the election timeouts in different peers don't always fire at the same time**, or else all peers will vote only for themselves and no one will become the leader.
- [x] The tester requires that the leader send **heartbeat RPCs no more than ten times per second**.
- [x] The tester requires your Raft **to elect a new leader within five seconds of the failure of the old leader** (if a majority of peers can still communicate). Remember, however, that leader election may require multiple rounds in case of a split vote (which can happen if packets are lost or if candidates unluckily choose the same random backoff times). You must pick election timeouts (and thus heartbeat intervals) that are short enough that it's very likely that an election will complete in less than five seconds even if it requires multiple rounds.
- [x] The paper's Section 5.2 mentions election timeouts in the range of 150 to 300 milliseconds. Such a range only makes sense if the leader sends heartbeats considerably more often than once per 150 milliseconds. Because the tester limits you to 10 heartbeats per second, you will have **to use an election timeout larger than the paper's 150 to 300 milliseconds, but not too large**, because then you may fail to elect a leader within five seconds.
- [x] You may find **Go's [rand](https://golang.org/pkg/math/rand/)** useful.
- [x] You'll need to write code that takes actions periodically or after delays in time. The easiest way to do this is to create **a goroutine with a loop that calls [time.Sleep()](https://golang.org/pkg/time/#Sleep); (see the `ticker()` goroutine that `Make()` creates for this purpose)**. Don't use Go's `time.Timer` or `time.Ticker`, which are difficult to use correctly.
- [x] The [Guidance page](https://pdos.csail.mit.edu/6.824/labs/guidance.html) has some tips on how to develop and debug your code.
- [x] If your code has trouble passing the tests, read the paper's Figure 2 again; the full logic for leader election is spread over multiple parts of the figure.
- [x] Don't forget **to implement `GetState()`.**
- [x] The tester calls your Raft's `rf.Kill()` when it is permanently shutting down an instance. **You can check whether `Kill()` has been called using `rf.killed()`.** You may want to do this in all loops, to avoid having dead Raft instances print confusing messages.
- [x] **Go RPC sends only struct fields whose names start with capital letters.** Sub-structures must also have capitalized field names (e.g. fields of log records in an array). The `labgob` package will warn you about this; don't ignore the warnings.



## Part 2B: log

### Hint

- [x] Run `git pull` to get the latest lab software.
- [x] Your first goal should be to pass `TestBasicAgree2B()`. 
  - [x] Start by implementing `Start()`, 
  - [x] then write the code to send and receive new log entries via `AppendEntries` RPCs, following Figure 2. 
  - [x] Send each newly committed entry on `applyCh` on each peer.
- [x] You will need to implement the election restriction (section 5.4.1 in the paper).
- [ ] One way to fail to reach agreement in the early Lab 2B tests is to hold repeated elections even though the leader is alive. 
  - [ ] Look for bugs in election timer management, 
  - [ ] or not sending out heartbeats immediately after winning an election.
- [ ] Your code may have 
  - [ ] loops that repeatedly check for certain events. 
    - [ ] Don't have these loops execute continuously without pausing, since that will slow your implementation enough that it fails tests. 
    - [ ] Use Go's [condition variables](https://golang.org/pkg/sync/#Cond), or insert a `time.Sleep(10 * time.Millisecond)` in each loop iteration.
- [ ] Do yourself a favor for future labs and write (or re-write) code that's clean and clear. For ideas, re-visit our the [Guidance page](https://pdos.csail.mit.edu/6.824/labs/guidance.html) with tips on how to develop and debug your code.
- [ ] **<u>*If you fail a test*</u>**, look over the code for the test in `config.go` and `test_test.go` to get a better understanding what the test is testing. `config.go` also illustrates how the tester uses the Raft API.



## Part 2C: persistence

- [ ] it will save and restore persistent state from a `Persister` object (see `persister.go`). 
  - [ ] Whoever calls `Raft.Make()` supplies a `Persister` that initially holds Raft's most recently persisted state (if any). 
  - [ ] Raft should initialize its state from that `Persister`, and should use it to save its persistent state each time the state changes. 
  - [ ] Use the `Persister`'s `ReadRaftState()` and `SaveRaftState()` methods.

### Task

- [x] Complete the functions `persist()` and `readPersist()` in `raft.go` by adding code to save and restore persistent state. 
  - [x] You will need to encode (or "serialize") the state as an array of bytes in order to pass it to the `Persister`. 
    - [x] Use the `labgob` encoder; see the comments in `persist()` and `readPersist()`. `labgob` is like Go's `gob` encoder but prints error messages if you try to encode structures with lower-case field names. 
  - [x] Insert calls to `persist()` at the points where your implementation changes persistent state. 
- [x] Once you've done this, and if the rest of your implementation is correct, you should pass all of the 2C tests.

### Hint

- [x] Run `git pull` to get the latest lab software.
- [x] The 2C tests are more demanding than those for 2A or 2B, and failures may be caused by problems in your code for 2A or 2B.
- [x] You will probably need the optimization that backs up nextIndex by more than one entry at a time. Look at the [extended Raft paper](https://pdos.csail.mit.edu/6.824/papers/raft-extended.pdf) starting at the bottom of page 7 and top of page 8 (marked by a gray line). The paper is vague about the details; you will need to fill in the gaps, perhaps with the help of the 6.824 Raft lecture notes.



## Part 2D: log compaction

- [ ] Section 7 of the [extended Raft paper](https://pdos.csail.mit.edu/6.824/papers/raft-extended.pdf) outlines the scheme; you will have to design the details.
- [ ] You may find it helpful to refer to the [diagram of Raft interactions](https://pdos.csail.mit.edu/6.824/notes/raft_diagram.pdf) to understand how the replicated service and Raft communicate.
- [x] Your Raft must provide the following function that the service can call with a serialized snapshot of its state: `Snapshot(index int, snapshot []byte)`
  - [x] In Lab 2D, the tester calls `Snapshot()` periodically. In Lab 3, you will write a key/value server that calls `Snapshot()`; the snapshot will contain the complete table of key/value pairs. The service layer calls `Snapshot()` on every peer (not just on the leader).
  - [x] The `index` argument indicates the highest log entry that's reflected in the snapshot. Raft should discard its log entries before that point. 
  - [x] You'll need to revise your Raft code to operate while storing only the tail of the log.
- [x] You'll need to implement the `InstallSnapshot` RPC discussed in the paper that allows a Raft leader to tell a lagging Raft peer to replace its state with a snapshot. 
  - [ ] You will likely need to think through how InstallSnapshot should interact with the state and rules in Figure 2.
- [x] When a follower's Raft code receives an InstallSnapshot RPC, it can use the `applyCh` to send the snapshot to the service in an `ApplyMsg`. The `ApplyMsg` struct definition already contains the fields you will need (and which the tester expects). 
  - [x] Take care that these snapshots only advance the service's state, and don't cause it to move backwards.
- [x] If a server crashes, it must restart from persisted data. Your Raft should persist both Raft state and the corresponding snapshot. Use `persister.SaveStateAndSnapshot()`, which takes separate arguments for the Raft state and the corresponding snapshot. If there's no snapshot, pass `nil` as the `snapshot` argument.

### Hint

- [x] `git pull` to make sure you have the latest software.
- [x] A good place to start is to modify your code to so that it is able to store just the part of the log starting at some index X. Initially you can set X to zero and run the 2B/2C tests. Then make `Snapshot(index)` discard the log before `index`, and set X equal to `index`. If all goes well you should now pass the first 2D test.
  - [x] You won't be able to store the log in a Go slice and use Go slice indices interchangeably with Raft log indices; you'll need to index the slice in a way that accounts for the discarded portion of the log.
- [x] Next: have the leader send an InstallSnapshot RPC if it doesn't have the log entries required to bring a follower up to date.
  - [x] Send the entire snapshot in a single InstallSnapshot RPC. Don't implement Figure 13's `offset` mechanism for splitting up the snapshot.

- [x] Raft must discard old log entries in a way that allows the Go garbage collector to free and re-use the memory; this requires that there be no reachable references (pointers) to the discarded log entries.
- [x] Even when the log is trimmed, your implemention still needs to properly send the term and index of the entry prior to new entries in `AppendEntries` RPCs; this may require saving and referencing the latest snapshot's `lastIncludedTerm/lastIncludedIndex` (consider whether this should be persisted).
- [ ] A reasonable amount of time to consume for the full set of Lab 2 tests (2A+2B+2C+2D) without `-race` is 6 minutes of real time and one minute of CPU time. When running with `-race`, it is about 10 minutes of real time and two minutes of CPU time.



# Lab 3: Fault-tolerant Key/Value Service

## TODO

- [x] raft返回leaderId

### Speed

- [x] 减少raft的并发

```shell
tian@Haotians-MBP raft % go test
Test (2A): initial election ...
  ... Passed --   4.0  3   56   18622    0
Test (2A): election after network failure ...
  ... Passed --   7.1  3  138   30416    0
Test (2A): multiple elections ...
  ... Passed --   8.0  7  538  124518    0
Test (2B): basic agreement ...
  ... Passed --   1.8  3   16    5248    3
Test (2B): RPC byte count ...
  ... Passed --   3.5  3   48  116748   11
Test (2B): agreement after follower reconnects ...
  ... Passed --   4.7  3   76   23696    7
Test (2B): no agreement if too many followers disconnect ...
  ... Passed --   5.7  5  156   41810    4
Test (2B): concurrent Start()s ...
  ... Passed --   1.8  3   18    5922    6
Test (2B): rejoin of partitioned leader ...
  ... Passed --   6.6  3  146   33369    4
Test (2B): leader backs up quickly over incorrect follower logs ...
  ... Passed --  29.0  5 2076 1701228  102
Test (2B): RPC counts aren't too high ...
  ... Passed --   3.1  3   40   13792   12
Test (2C): basic persistence ...
  ... Passed --   7.6  3  106   28404    6
Test (2C): more persistence ...
  ... Passed --  22.2  5  968  226422   16
Test (2C): partitioned leader and one follower crash, leader restarts ...
  ... Passed --   3.8  3   36   10673    4
Test (2C): Figure 8 ...
  ... Passed --  36.2  5  428   93832   10
Test (2C): unreliable agreement ...
  ... Passed --  11.1  5  408  144603  246
Test (2C): Figure 8 (unreliable) ...
  ... Passed --  36.4  5 2743 5635659  479
Test (2C): churn ...
  ... Passed --  16.5  5  544  357145  115
Test (2C): unreliable churn ...
  ... Passed --  16.5  5  256   72814   30
Test (2D): snapshots basic ...
  ... Passed --   8.2  3  136   55842  215
Test (2D): install snapshots (disconnect) ...
  ... Passed --  60.5  3 1281  462659  345
Test (2D): install snapshots (disconnect+unreliable) ...
  ... Passed --  74.3  3 1643  535476  326
Test (2D): install snapshots (crash) ...
  ... Passed --  51.2  3  732  331236  335
Test (2D): install snapshots (unreliable+crash) ...
  ... Passed --  60.2  3  846  356597  307
Test (2D): crash and restart all servers ...
  ... Passed --  19.6  3  282   96426   62
PASS
ok      6.824/raft      499.867s


```



## TODO_END

### Introduction

- [ ] In this lab you will build a fault-tolerant key/value storage service using your Raft library
  - [ ] Your key/value service will be a replicated state machine, consisting of several key/value servers that use Raft for replication.
- [ ] Clients can send three different RPCs to the key/value service: `Put(key, value)`, `Append(key, arg)`, and `Get(key)`. 
  - [ ] The service maintains a simple database of key/value pairs. Keys and values are strings. 
    - [ ] `Put(key, value)` replaces the value for a particular key in the database, 
    - [ ] `Append(key, arg)` appends arg to key's value, and 
    - [ ] `Get(key)` fetches the current value for the key. 
    - [ ] A `Get` for a non-existent key should return an empty string. 
    - [ ] An `Append` to a non-existent key should act like `Put`. 
  - [ ] Each client talks to the service through a `Clerk` with Put/Append/Get methods. A `Clerk` manages RPC interactions with the servers.
- [ ] **<u>*You should review the [extended Raft paper](https://pdos.csail.mit.edu/6.824/papers/raft-extended.pdf), in particular Sections 7 and 8.*</u>**

### Getting Started

- [ ] We supply you with skeleton code and tests in `src/kvraft`. 
- [ ] You will need to modify `kvraft/client.go`, `kvraft/server.go`, and perhaps `kvraft/common.go`.

## Part A: Key/value service without snapshots

### A1

- Each of your key/value servers ("*kvservers*") will have an associated *Raft* peer. 
  - *Clerks* send `Put()`, `Append()`, and `Get()` RPCs to the *kvserver* whose associated *Raft* is the leader. The *kvserver* code submits the Put/Append/Get operation to *Raft*, so that the *Raft* log holds a sequence of Put/Append/Get operations. All of the *kvservers* execute operations from the *Raft* log in order, applying the operations to their key/value databases; the intent is for the servers to maintain identical replicas of the key/value database.
  - A `Clerk` sometimes doesn't know which kvserver is the Raft leader. If the `Clerk` sends an RPC to the wrong kvserver, or if it cannot reach the kvserver, the `Clerk` should re-try by sending to a different kvserver.
  - If the key/value service commits the operation to its Raft log (and hence applies the operation to the key/value state machine), the leader reports the result to the `Clerk` by responding to its RPC. If the operation failed to commit (for example, if the leader was replaced), the server reports an error, and the `Clerk` retries with a different server.
  - Your kvservers should not directly communicate; they should only interact with each other through Raft.

### Task1

- [ ] Your first task is to implement a solution that works when there are no dropped messages, and no failed servers.
- [ ] You'll need to 
  - [x] add RPC-sending code to the Clerk Put/Append/Get methods in `client.go`, and 
  - [x] implement `PutAppend()` and `Get()` RPC handlers in `server.go`. 
    - [x] These handlers should enter an `Op` in the Raft log using `Start()`; 
    - [x] you should fill in the `Op` struct definition in `server.go` so that it describes a Put/Append/Get operation. 
    - [x] Each server should execute `Op` commands as Raft commits them, i.e. as they appear on the `applyCh`. 
    - [x] An RPC handler should notice when Raft commits its `Op`, and then reply to the RPC.
- [x] You have completed this task when you **reliably** pass the first test in the test suite: "One client".

### Hint1

- [x] After calling `Start()`, your kvservers will need to wait for Raft to complete agreement. Commands that have been agreed upon arrive on the `applyCh`. 
  - [x] Your code will need to keep reading `applyCh` while `PutAppend()` and `Get()` handlers submit commands to the Raft log using `Start()`. Beware of deadlock between the kvserver and its Raft library.
- [x] You are allowed to add fields to the Raft `ApplyMsg`, and to add fields to Raft RPCs such as `AppendEntries`, however this should not be necessary for most implementations.
- [x] A kvserver should not complete a `Get()` RPC if it is not part of a majority (so that it does not serve stale data). A simple solution is to enter every `Get()` (as well as each `Put()` and `Append()`) in the Raft log. You don't have to implement the optimization for read-only operations that is described in Section 8.
- [ ] ~~It's best to add locking from the start because the need to avoid deadlocks sometimes affects overall code design. Check that your code is race-free using `go test -race`.~~

### A2

- One problem you'll face is that a `Clerk` may have to send an RPC multiple times until it finds a kvserver that replies positively. 
  - If a leader fails just after committing an entry to the Raft log, the `Clerk` may not receive a reply, and thus may re-send the request to another leader. 
  - Each call to `Clerk.Put()` or `Clerk.Append()` should result in just a single execution, so you will have to ensure that the re-send doesn't result in the servers executing the request twice.

### Task2

- [ ] Add code to handle failures, and to cope with duplicate `Clerk` requests, including situations where 
  - [ ] the `Clerk` sends a request to a kvserver leader in one term, times out waiting for a reply, and re-sends the request to a new leader in another term. The request should execute just once. 
  - [ ] Your code should pass the `go test -run 3A -race` tests.

### Hint

- [x] Your solution needs to handle a leader that has called Start() for a Clerk's RPC, but loses its leadership before the request is committed to the log. In this case you should arrange for the Clerk to re-send the request to other servers until it finds the new leader. 
  - [x] One way to do this is for the server to detect that it has lost leadership, by noticing 
    - [x] that a different request has appeared at the index returned by Start(), 
    - [ ] ~~or that Raft's term has changed.~~ 
  - [x] If the ex-leader is partitioned by itself, it won't know about new leaders; but any client in the same partition won't be able to talk to a new leader either, so it's OK in this case for the server and client to wait indefinitely until the partition heals.
- [x] You will probably have to modify your Clerk to remember which server turned out to be the leader for the last RPC, and send the next RPC to that server first. 
  - [x] This will avoid wasting time searching for the leader on every RPC, which may help you pass some of the tests quickly enough.
- [x] You will need to uniquely identify client operations to ensure that the key/value service executes each one just once.
- [ ] ~~Your scheme for duplicate detection should free server memory quickly, for example by having each RPC imply that the client has seen the reply for its previous RPC. It's OK to assume that a client will make only one call into a Clerk at a time.~~



# Lab 4: Sharded Key/Value Service

## Part A: The Shard controller (30 points)

- [x] First you'll implement the shard controller, in `shardctrler/server.go` and `client.go`. 
- [x] When you're done, you should pass all the tests in the `shardctrler` directory

### Task

- The shardctrler manages a sequence of numbered configurations. 
  - Each configuration describes a set of replica groups and an assignment of shards to replica groups. 
  - Whenever this assignment needs to change, the shard controller creates a new configuration with the new assignment. 
  - Key/value clients and servers contact the shardctrler when they want to know the current (or a past) configuration.
- Your implementation must support the RPC interface described in `shardctrler/common.go`, which consists of `Join`, `Leave`, `Move`, and `Query` RPCs. These RPCs are intended to allow an administrator (and the tests) to control the shardctrler: to add new replica groups, to eliminate replica groups, and to move shards between replica groups.
  - The `Join` RPC is used by an administrator to add new replica groups. Its argument is a set of mappings from unique, non-zero replica group identifiers (GIDs) to lists of server names. The shardctrler should react by creating a new configuration that includes the new replica groups. 
    - The new configuration should divide the shards as evenly as possible among the full set of groups, and should move as few shards as possible to achieve that goal. 
    - The shardctrler should allow re-use of a GID if it's not part of the current configuration (i.e. a GID should be allowed to Join, then Leave, then Join again).
  - The `Leave` RPC's argument is a list of GIDs of previously joined groups. The shardctrler should create a new configuration that does not include those groups, and that assigns those groups' shards to the remaining groups. 
    - The new configuration should divide the shards as evenly as possible among the groups, and should move as few shards as possible to achieve that goal.
  - The `Move` RPC's arguments are a shard number and a GID. The shardctrler should create a new configuration in which the shard is assigned to the group. 
    - The purpose of `Move` is to allow us to test your software. A `Join` or `Leave` following a `Move` will likely un-do the `Move`, since `Join` and `Leave` re-balance.
  - The `Query` RPC's argument is a configuration number. The shardctrler replies with the configuration that has that number. 
    - If the number is -1 or bigger than the biggest known configuration number, the shardctrler should reply with the latest configuration. 
    - The result of `Query(-1)` should reflect every `Join`, `Leave`, or `Move` RPC that the shardctrler finished handling before it received the `Query(-1)` RPC.
- The very first configuration should be numbered zero. It should contain no groups, and all shards should be assigned to GID zero (an invalid GID). 
  - The next configuration (created in response to a `Join` RPC) should be numbered 1, &c. 
  - There will usually be significantly more shards than groups (i.e., each group will serve more than one shard), in order that load can be shifted at a fairly fine granularity.

### Task

- [x] Your task is to implement the interface specified above in `client.go` and `server.go` in the `shardctrler/` directory. 
  - [x] Your shardctrler must be fault-tolerant, using your Raft library from Lab 2/3. 
  - [x] Note that we will re-run the tests from Lab 2 and 3 when grading Lab 4, so make sure you do not introduce bugs into your Raft implementation. 
- [x] You have completed this task when you pass all the tests in `shardctrler/`.

### Hint

- [x] Start with a stripped-down copy of your kvraft server.
- [x] You should implement duplicate client request detection for RPCs to the shard controller. 
  - [x] The shardctrler tests don't test this, but the shardkv tests will later use your shardctrler on an unreliable network; you may have trouble passing the shardkv tests if your shardctrler doesn't filter out duplicate RPCs.
- [x] The code in your state machine that performs the shard rebalancing needs to be deterministic. 
  - [x] In Go, map iteration order is [not deterministic](https://blog.golang.org/maps#TOC_7.).
- [x] Go maps are references. If you assign one variable of type map to another, both variables refer to the same map. Thus if you want to create a new `Config` based on a previous one, you need to create a new map object (with `make()`) and copy the keys and values individually.
- [x] The Go race detector (go test -race) may help you find bugs.



## Part B: Sharded Key/Value Server

- Each shardkv server operates as part of a replica group. Each replica group serves `Get`, `Put`, and `Append` operations for some of the key-space shards. 
  - Use `key2shard()` in `client.go` to find which shard a key belongs to. Multiple replica groups cooperate to serve the complete set of shards. A single instance of the `shardctrler` service assigns shards to replica groups; 
  - when this assignment changes, replica groups have to hand off shards to each other, while ensuring that clients do not see inconsistent responses.
- Your storage system must provide a linearizable interface to applications that use its client interface. 
  - That is, completed application calls to the `Clerk.Get()`, `Clerk.Put()`, and `Clerk.Append()` methods in `shardkv/client.go` must appear to have affected all replicas in the same order.
  - A `Clerk.Get()` should see the value written by the most recent `Put`/`Append` to the same key. This must be true even when `Get`s and `Put`s arrive at about the same time as configuration changes.
- Each of your shards is only required to make progress when a majority of servers in the shard's Raft replica group is alive and can talk to each other, and can talk to a majority of the `shardctrler` servers. 
  - Your implementation must operate (serve requests and be able to re-configure as needed) even if a minority of servers in some replica group(s) are dead, temporarily unavailable, or slow.
- A shardkv server is a member of only a single replica group. The set of servers in a given replica group will never change.
- We supply you with `client.go` code that sends each RPC to the replica group responsible for the RPC's key. 
  - It re-tries if the replica group says it is not responsible for the key; in that case, the client code asks the shard controller for the latest configuration and tries again. 
  - You'll have to modify client.go as part of your support for dealing with duplicate client RPCs, much as in the kvraft lab.

### Note

- [ ] Your server should not call the shard controller's `Join()` handler. The tester will call `Join()` when appropriate.

### Task

- Your first task is to pass the very first shardkv test. In this test, there is only a single assignment of shards, so your code should be very similar to that of your Lab 3 server. 
  - The biggest modification will be to have your server detect when a configuration happens and start accepting requests whose keys match shards that it now owns.

### Task

- Implement shard migration during configuration changes. 
  - Make sure that all servers in a replica group do the migration at the same point in the sequence of operations they execute, so that they all either accept or reject concurrent client requests. 
  - You should focus on passing the second test ("join then leave") before working on the later tests. 
  - You are done with this task when you pass all tests up to, but not including, `TestDelete`.

### Note

- [ ] Your server will need to periodically poll the shardctrler to learn about new configurations. The tests expect that your code polls roughly every 100 milliseconds; more often is OK, but much less often may cause problems.
- [ ] Servers will need to send RPCs to each other in order to transfer shards during configuration changes. The shardctrler's `Config` struct contains server names, but you need a `labrpc.ClientEnd` in order to send an RPC. You should use the `make_end()` function passed to `StartServer()` to turn a server name into a `ClientEnd`. `shardkv/client.go` contains code that does this.

### Hint

- [ ] Add code to `server.go` to periodically fetch the latest configuration from the shardctrler, and add code to reject client requests if the receiving group isn't responsible for the client's key's shard. You should still pass the first test.
- [ ] Your server should respond with an `ErrWrongGroup` error to a client RPC with a key that the server isn't responsible for (i.e. for a key whose shard is not assigned to the server's group). Make sure your `Get`, `Put`, and `Append` handlers make this decision correctly in the face of a concurrent re-configuration.
- [ ] Process re-configurations one at a time, in order.
- [ ] If a test fails, check for gob errors (e.g. "gob: type not registered for interface ..."). Go doesn't consider gob errors to be fatal, although they are fatal for the lab.
- [ ] You'll need to provide at-most-once semantics (duplicate detection) for client requests across shard movement.
- [ ] Think about how the shardkv client and server should deal with `ErrWrongGroup`. Should the client change the sequence number if it receives `ErrWrongGroup`? Should the server update the client state if it returns `ErrWrongGroup` when executing a `Get`/`Put` request?
- [ ] After a server has moved to a new configuration, it is acceptable for it to continue to store shards that it no longer owns (though this would be regrettable in a real system). This may help simplify your server implementation.
- [ ] When group G1 needs a shard from G2 during a configuration change, does it matter at what point during its processing of log entries G2 sends the shard to G1?
- [ ] You can send an entire map in an RPC request or reply, which may help keep the code for shard transfer simple.
- [ ] If one of your RPC handlers includes in its reply a map (e.g. a key/value map) that's part of your server's state, you may get bugs due to races. The RPC system has to read the map in order to send it to the caller, but it isn't holding a lock that covers the map. Your server, however, may proceed to modify the same map while the RPC system is reading it. The solution is for the RPC handler to include a copy of the map in the reply.
- [ ] If you put a map or a slice in a Raft log entry, and your key/value server subsequently sees the entry on the `applyCh` and saves a reference to the map/slice in your key/value server's state, you may have a race. Make a copy of the map/slice, and store the copy in your key/value server's state. The race is between your key/value server modifying the map/slice and Raft reading it while persisting its log.
- [ ] During a configuration change, a pair of groups may need to move shards in both directions between them. If you see deadlock, this is a possible source.



### 2nd design

- state:
  - 'kvMap'
  - 'lastOps'
  - 'validShards'
  - 'handOffs'
- state change:
  - [x] start
    - if first time
      - get first config and add shards to 'validShards'
  - [x] hand off shards
    - delete shards from 'validShards'
    - hands off shards to their current group
    - hands off related 'lastOps'
  - [x] receive shards
    - add shards to 'validShards'
    - update 'kvMap'
    - update 'lastOps'
- operations
  - [x] handoffShards(shards, gid)
    - retry by polling

  - [x] addShards(shards, kvMap, lastOps)
- RPC
  - [x] HandOffShards
- details
  - [x] where to check 'validShards'
    - [x] receive client RPCs (return fast)
    - [x] receive ApplyMsgs + waitApply
      - linearization: wrong request get in the log but after reconfiguration in the log





### 3rd Design

- State:

  - storage

    - `kvMap`

    - `lastOps`

    - `validShards`

  - state

    - `configNum`

    - `toHandOff [int][]int // configNum -> shards`

- State change:

  - firstConfig
    - `if configNum == 0`
      - get first config and add shards to `validShards`
      - `configNum++`
  - ticker
    - pollConfig
      - deleteShards
      - handOff shards in `toHandOff[configNum + 1]`
  - deleteShards
    - delete shards from `validShards`
    - add shards to `toHandOff[configNum + 1]`
  - addShards
    - add shards to `validShards`
    - update `kvMap`
    - update `lastOps`
    - 
  - hand off shards
    - delete shards from 'validShards'
    - hands off shards to their current group
    - hands off related 'lastOps'
  - receive shards
    - add shards to 'validShards'
    - update 'kvMap'
    - update 'lastOps'



























