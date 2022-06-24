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





























