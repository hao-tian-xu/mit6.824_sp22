# Students' Guide to Raft

Posted on Mar 16, 2016

### Background

- When *delayed messages, network partitions, and failed servers* are introduced, each and every if, but, and and, become crucial. 
  - In particular, there are a number of bugs that we see repeated over and over again, simply due to misunderstandings or oversights when reading the paper.

### Implementing Raft

- In fact, Figure 2 is extremely precise, and every single statement it makes should be treated, in specification terms, as **MUST**, not as **SHOULD**.



## *Thinking*

- 或许可以做一个分析Raft的Diagram



# Raft Locking Advice

### Locking Rules

- Rule 1: Whenever you have data that more than one goroutine uses, and at least one goroutine might modify the data, the goroutines should use locks to prevent simultaneous use of the data.

- Rule 2: Whenever code makes a sequence of modifications to shared data, and other goroutines might malfunction if they looked at the data midway through the sequence, you should use a lock around the whole sequence. e.g.

  - ```go
    rf.mu.Lock()
    rf.currentTerm += 1
    rf.state = Candidate
    rf.mu.Unlock()
    ```

  - It would be a mistake for another goroutine to see either of these updates alone (i.e. the old state with the new term, or the new term with the old state).

- Rule 3: Whenever code does a sequence of reads of shared data (or reads and writes), and would malfunction if another goroutine modified the data midway through the sequence, you should use a lock around the whole sequence. e.g.

  - ```go
    rf.mu.Lock()
    if args.Term > rf.currentTerm {
    	rf.currentTerm = args.Term
    }
    rf.mu.Unlock()
    ```

  - Real Raft code would need to use longer critical sections than these examples; for example, a Raft RPC handler should probably hold the lock for the entire handler.

- Rule 4: It's usually a bad idea to hold a lock while doing anything that might wait: reading a Go channel, sending on a channel, waiting for a timer, calling time.Sleep(), or sending an RPC (and waiting for the reply).

  - Code that waits should first release locks. If that's not convenient, *sometimes it's useful to create a separate goroutine to do the wait*.

- Rule 5: Be careful about assumptions across a drop and re-acquire of a lock. One place this can arise is when avoiding waiting with locks held. e.g.

  - ```go
    rf.mu.Lock()
    rf.currentTerm += 1
    rf.state = Candidate
    for <each peer> {
      go func() {
        rf.mu.Lock()
        args.Term = rf.currentTerm
        rf.mu.Unlock()
        Call("Raft.RequestVote", &args, ...)
        // handle the reply...
      } ()
    }
    rf.mu.Unlock()
    ```

  - The code sends each RPC in a separate goroutine. It's incorrect because args.Term may not be the same as the rf.currentTerm at which the surrounding code decided to become a Candidate.

    - One way to fix this is for the created goroutine to use a copy of rf.currentTerm made while the outer code holds the lock.
    - Similarly, reply-handling code after the Call() must re-check all relevant assumptions after re-acquiring the lock; for example, it should check that rf.currentTerm hasn't changed since the decision to become a candidate.

### Approach

- A more pragmatic approach
  - You have concurrency forced on you when 
    - the RPC system creates goroutines to execute RPC handlers, and 
    - because you need to send RPCs in separate goroutines to avoid waiting.
  - You can effectively eliminate this concurrency by identifying all places where goroutines start (RPC handlers, background goroutines you create in Make(), &c), *acquiring the lock at the very start of each goroutine, and only releasing the lock when that goroutine has completely finished and returns*.
    - With no parallel execution, it's hard to violate Rules 1, 2, 3, or 5.
  - Rule 4 is likely to be a problem. So the next step is to find places where the code waits, and to add lock releases and re-acquires (and/or goroutine creation) as needed, being careful to re-establish assumptions after each re-acquire.
- You may find this process easier to get right than directly identifying sequences that must be locked for correctness.
- As an aside, what this approach sacrifices is any opportunity for better performance via parallel execution on multiple cores. On the other hand, there is not much opportunity for CPU parallelism within a single Raft peer.

## *Thinking*

- 列举了Raft中使用Lock的规则
- 提出了一个简单的使用Lock的方法，会牺牲一些并行运算的可能，但作者认为一个单独的Raft peer中本身不存在太多并行的机会



# Raft Structure Advice

- A Raft instance has to 
  - deal with the arrival of external events (`Start()` calls, `AppendEntries` and `RequestVote` RPCs, and RPC replies), and it has to 
  - execute periodic tasks (elections and heart-beats). 
- There are many ways to structure your Raft code to manage these activities; this document outlines a few ideas.

### Structure ideas

- Each Raft instance has 
  - a bunch of state (the log, the current index, &c) 
    - which must be updated in response to events arising in concurrent goroutines. 
  - The Go documentation points out that the goroutines can perform the updates directly using shared data structures and locks, or by passing messages on channels. 
    - Experience suggests that for Raft it is most straightforward to use *<u>shared data and locks</u>*.
- A Raft instance has 
  - two time-driven activities: 
    - <u>*the leader must send heart-beats*</u>, and 
    - <u>*others must start an election if*</u> too much time has passed since hearing from the leader. 
  - It's probably best *<u>to drive each of these activities with a dedicated long-running goroutine</u>*, rather than combining multiple activities into a single goroutine.
- <u>*The management of the election timeout*</u> is a common source of headaches. 
  - Perhaps <u>*the simplest plan*</u> is to 
    - maintain a variable in the Raft struct containing the last time at which the peer heard from the leader, and to 
    - have the election timeout goroutine periodically check to see whether the time since then is greater than the timeout period. 
  - It's easiest to use time.Sleep() with a small constant argument to drive the periodic checks. Don't use time.Ticker and time.Timer; they are tricky to use correctly.
- You'll want to have 
  - <u>*a separate long-running goroutine that sends committed log entries in order on the applyCh*</u>. 
    - It must be separate, since sending on the applyCh can block; and 
    - it must be a single goroutine, since otherwise it may be hard to ensure that you send log entries in log order. 
  - The code that advances commitIndex will need to kick the apply goroutine; it's probably easiest to use a condition variable (Go's sync.Cond) for this.
- <u>*Each RPC should probably be sent (and its reply processed) in its own goroutine*</u>, 
  - for two reasons: 
    - so that unreachable peers don't delay the collection of a majority of replies, and 
    - so that the heartbeat and election timers can continue to tick at all times. 
  - It's easiest to do the RPC reply processing in the same goroutine, rather than sending reply information over a channel.
- Keep in mind that <u>*the network can delay RPCs and RPC replies*</u>, and when you send concurrent RPCs, <u>*the network can re-order requests and replies*</u>. 
  - Figure 2 is pretty good about pointing out places where RPC handlers have to be careful about this 
    - (e.g. an RPC handler should ignore RPCs with old terms). 
  - Figure 2 is not always explicit about RPC reply processing. 
    - The leader has to be careful when processing replies; it 
      - must check that the term hasn't changed since sending the RPC, and 
      - must account for the possibility that replies from concurrent RPCs to the same follower have changed the leader's state (e.g. nextIndex).















































