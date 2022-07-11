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
  - [ ] This way a peer will learn who is the leader, if there is already a leader, or become the leader itself. 
  - [x] Implement the `RequestVote()` RPC handler so that servers will vote for one another.
- [x] **To implement heartbeats, define an `AppendEntries` RPC struct** (though you may not need all the arguments yet), and 
  - [x] have the leader **send them out periodically**. 
  - [x] **Write an `AppendEntries` RPC handler method** that resets the election timeout so that other servers don't step forward as leaders when one has already been elected.
- [x] Make sure **the election timeouts in different peers don't always fire at the same time**, or else all peers will vote only for themselves and no one will become the leader.
- [x] The tester requires that the leader send **heartbeat RPCs no more than ten times per second**.
- [ ] The tester requires your Raft **to elect a new leader within five seconds of the failure of the old leader** (if a majority of peers can still communicate). Remember, however, that leader election may require multiple rounds in case of a split vote (which can happen if packets are lost or if candidates unluckily choose the same random backoff times). You must pick election timeouts (and thus heartbeat intervals) that are short enough that it's very likely that an election will complete in less than five seconds even if it requires multiple rounds.
- [ ] The paper's Section 5.2 mentions election timeouts in the range of 150 to 300 milliseconds. Such a range only makes sense if the leader sends heartbeats considerably more often than once per 150 milliseconds. Because the tester limits you to 10 heartbeats per second, you will have **to use an election timeout larger than the paper's 150 to 300 milliseconds, but not too large**, because then you may fail to elect a leader within five seconds.
- [x] You may find **Go's [rand](https://golang.org/pkg/math/rand/)** useful.
- [x] You'll need to write code that takes actions periodically or after delays in time. The easiest way to do this is to create **a goroutine with a loop that calls [time.Sleep()](https://golang.org/pkg/time/#Sleep); (see the `ticker()` goroutine that `Make()` creates for this purpose)**. Don't use Go's `time.Timer` or `time.Ticker`, which are difficult to use correctly.
- [ ] The [Guidance page](https://pdos.csail.mit.edu/6.824/labs/guidance.html) has some tips on how to develop and debug your code.
- [ ] If your code has trouble passing the tests, read the paper's Figure 2 again; the full logic for leader election is spread over multiple parts of the figure.
- [ ] Don't forget **to implement `GetState()`.**
- [ ] The tester calls your Raft's `rf.Kill()` when it is permanently shutting down an instance. **You can check whether `Kill()` has been called using `rf.killed()`.** You may want to do this in all loops, to avoid having dead Raft instances print confusing messages.
- [ ] **Go RPC sends only struct fields whose names start with capital letters.** Sub-structures must also have capitalized field names (e.g. fields of log records in an array). The `labgob` package will warn you about this; don't ignore the warnings.



## Part 2B: log

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

































