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



# L6. Debugging

## Introduction

### Why hard

- The algorithms are difficult to comprehend.
  - Details are easy to misunderstand or misinterpret, leaving your code with subtle bugs.
- Distributed systems are highly parallel, with many possible interleavings.
  - You have to consider all of the ways different threads and machines can interact.
- The activities in a distributed system are often not cleanly separated.
  - It can be hard to isolate erroneous behavior from the vast quantities of correct behavior occurring around it.

### Objectives

- A distributed system used by a million users that functions correctly 99.9% of the time will still be broken for a thousand users.
- **You will learn the most if you refine your code until all the bugs are gone.**
- We only give you credit for a test case if you pass it every time.

### What are the labs like?

- You are likely to spend more time debugging than writing code
- But debugging is more about **quality** than **time spent**.
  - **If you aren’t making progress, stop and try a new approach!**
- If you run out of ideas and get stuck, please come to office hours.
  - **We do not want you to suffer alone.** We will help you figure out the right next steps!

### Where do bugs come from?

- Mistakes and typos in the code you wrote. (Common.)
- Hardware glitches and compiler bugs. (Rare.)
- The worst bugs often come from **incorrect or incomplete mental models**.
  - Strange behaviors in systems can emerge from **unexpected interactions** of different components.
  - “A system is [something that has] the ability to fail in a way humans find surprising.” - https://twitter.com/SwiftOnSecurity/status/1484962240465932304
  -  If you don’t quite realize something about how a system works, you may write incorrect code for it, and **you might not realize that the code is incorrect, even when you look at it.**



## Terminology

- We debug because we’ve observed a **failure** and want to find the **fault**, so that we can fix it and prevent the **failure** from happening again. We can find the **fault** by tracing backwards through the series of **errors** that led to the **failure**.
  - FAULT: The original underlying cause of an error
    - Such as a typo, incorrect logic, a wrong assumption, or a hardware glitch.
  - ERROR: Any specific state in the system that is not what it should be.
    - Such as a variable with the wrong value or a function getting called when it shouldn’t.
  - FAILURE: The final visible malfunction of a system.
    - Such as a crash or a failure of a test case.

### Types of errors

- **LATENT ERROR**: an error where something is **invisibly wrong**.
  - These errors may silently propagate and lead to further errors.
- **MASKED ERROR**: an error that becomes **unimportant by happenstance**.
  - From the demo earlier: imagine if our reduce worker ignored any intermediate files it couldn't find, and it just so happened that the map tasks that weren't done executing didn't contain any key/value pairs.
  - It would still have been an error to assign the reduce task early, but it wouldn't matter!
- **OBSERVABLE ERROR**: an error that has been **surfaced** to you.
  - An observable error is apparent in the output of a program, possibly as an error message, an unexpected message, the lack of an expected message, or incorrect data in an intermediate file.

### The error model

<img src="image.assets/Screen Shot 2022-07-04 at 11.12.07.png" alt="Screen Shot 2022-07-04 at 11.12.07" style="zoom:25%;" />



## Fault Interval

### Methodical debugging: fault intervals

- A **fault interval** is an interval of time in which a fault must have occurred.
  - Initial fault interval: [ran the test script, observed a failure)
  - The fault must have occurred in this interval.
  - To find it, we shrink the length of the interval until there’s only a single line of code that could be at fault.
    - Pushing the start forward can only be an approximation: just because an intermediate state appears error-free, that doesn’t mean it is!
    - Pulling the end backward is easier: it is always the moment of the earliest observed error.
  - Every time we observe an error inside the fault interval, we can shrink it!

### How do we narrow down the fault interval?

- We add **instrumentation** to <u>turn latent errors inside the fault interval into observable errors</u>.
- Instrumentation: the parts of your code that let you detect particular errors.
- Examples:
  - **Assertions** and **validations** that check a piece of state, ensure that it has the expected value, and panic (or at least print) if it's wrong.
  - **Print statements** that display values, so that they can be manually analyzed.
- Also, consider using your own **test cases** to validate certain slices of code.
  - These can help you identify shorter fault intervals, which will be easier to debug!
  - <u>We provide the grading set, and you are welcome to copy them as a basis for your own!</u>
  - You can augment our **opaque testing** with **clear-box testing**, by examining internal state.
- Consider different approaches to solve different problems. 
  - **Assertions and validations** are effective when it is easy to check if something is correct, because they only provide a signal when tripped, and don’t obscure other information. 
  - **Print statements** are applicable to many situations, but can easily become overwhelming. 
  - **Test cases** can help you reduce the length of the program’s execution.

### The debugging “main loop”

1. Identify the earliest observable error (which is the end of the fault interval)
2. Formulate a **hypothesis** about the **most proximate cause** of the error.
3. Add instrumentation to test that hypothesis and run your code again.
4. **If false**: go back to step 2 and come up with a new hypothesis.
5. **If true**: you have a new earliest observable error. Continue from step 1 !

### Tips on following the main loop

- If you're struggling to make progress, **start writing down notes**!
  - Write down every time you find an earlier first observable error.
  - Write down your hypotheses, and their results.
- If your hypotheses aren't turning out to be true, **make them more specific**!
  - Eventually, you end up at the level of "why does this variable equal $X$ instead of $Y$ ?"

### Optimization: error intervals

- Situation: We have a variable that holds a value that it should not hold.
- We can often diagnose a value error by **tracing the value backward**.
- We can apply **binary search** to repeatedly bisect the flow of the variable.

### A note on multiple faults

- **This doesn’t need to change our approach**: we still keep debugging, identifying faults one at a time and fixing them, until our code stops failing.
- Remember: **if you fix a fault, and your code still fails, that doesn’t necessarily mean you were wrong about the fault!** There might be multiple faults involved!
  - The question to ask if this happens is – does this fault still lead to an error? And if not, what other source might the error have?

### Dealing with intermittent bugs

- In our labs, you are likely to encounter bugs that only appear rarely.
- Solution: **run your code many times**, until it fails at least once, while **printing out everything that could be relevant**.
- Afterwards, use tools to help you review different **slices** of the output. 
  - Use tools like grep, or your own tools, to search the output to answer specific questions.
- You can make lots of progress debugging from just a single execution. You only have to rerun when you want to add new information to your output!
  - **The more intermittent the bug, the more information you probably want to include!**



## Logging

### Logging to a file

- `$ go test -race`
  - This displays the output on your screen directly.
- `$ go test -race &> output.log`
  - This puts all the output into “output.log” so that you can review it later.
- `$ go test -race |& tee output.log`
  - This puts the output into the log AND displays it on your screen.
- `$ grep "an important string" output.log`
  - This searches through your log for only lines containing “an important string.” This can help you avoid scrolling through irrelevant information.

### Using format strings

- Consider using format strings to print concise single-line messages
  - `log.Printf(“[MODULE] Thing happened. MV=%v, MOV=%v\n”, my_variable, my_other_variable);`

### Questions to ask about print statements

- **How much detail do I need?**
  - More details makes instrumentation more broadly useful. Less helps avoid distraction!
- **Can I make it easy to adjust the focus and detail level later?**
  - You can define a constant for each module, and set it to true or false based on whether you want that module to print output. You can also filter out irrelevant detail with a tool!
- **What format should my print statements take?**
  - You may have to scroll through 1000 s of lines of output. What format is best for you?
  - Consider text colors, columns, consistent formats, code words, and symbols!

### Print statements may have timing effects

- Unfortunately, sometimes printing lots of information can actually mask or unmask bugs in your code!
  - This is because print statements can be slow, and so they can affect timing.
- Dealing with this requires some creativity.
  - You might write your logging messages to an array, and print them out in the background.
  - You might focus on using assertions and validations, which can execute faster in the common case.
  - You might try to inject or eliminate some other source of variability to compensate.
  - Or you might switch to trying to track down why inserting a delay on a certain line leads to success or failure!



## Further Advice

### Design for debugging

- You will spend lots of time debugging in 6.824, so you should consider ***debuggability*** as a primary goal when you write your code!
- Some ideas you might consider:
  - Using **assertions** everywhere in your code to keep errors from snowballing.
  - Building **abstractions** around debugging, such as by having all RPCs go through a single method, and always printing out the request and reply every time.
  - Printing out **specially-formatted logs**, so that you can <u>filter them or put them into columns</u>.
  - **Minimizing the number of goroutines** that you run, **and the number of times you use locks + channels**, so that there are fewer possible interleavings of your code.
- Be creative! Writing debuggable code can be difficult, so be prepared to revise your approach as the semester goes on!

### Understand whose code is in scope

- **All code is in scope for debugging.**
- You may need to read, understand, and instrument any piece of code. 
  - **Yes, even if we provided it!**
- If you're failing a test case and don't know why, **read that test case!**
- If you don't understand a piece of code we provided - **ask!**

### Tips on avoiding pitfalls

- Once you understand the code you've written, continuing to reread it to try to find bugs may not be effective!
- Just because a piece of code looks like it's correct doesn't mean it is!
- Just because you wrote a piece of code recently doesn't mean it's the most likely place to find a fault!
- **If you make changes to your code before you're confident that you understand what's wrong with it, you might make it worse!**

### Tweaking timeouts

- In the Raft labs, we ask you to decide on timeouts for certain behaviors.
  - Like elections and heartbeats.
- There are a wide range of timeouts that will let you pass the test cases.
  - You may need to try a few options!
- However: timeouts also impact the execution flow of a test case.
  - Changing timeouts can cause unrelated errors to become masked or unmasked!
- If you're spending a lot of time fiddling with timeouts, you're probably on the wrong track!
  - Oftentimes, this is simply obscuring or unobscuring the underlying fault.

## Wrap-up

- Debugging is challenging, but you can learn to do it well.
- Follow a **methodical** process to debug.
- **Experiment** with new approaches to debugging!
- **Most importantly: if you're spending hours debugging and don't get anywhere, that means you should try a different approach!**

### Further Resources

- Blog post from a former TA: [Debugging by Pretty Printing](https://blog.josejg.com/debugging-pretty/) 
  - If you don’t already know how to effectively filter down and view very large quantities of output from a program, please read this! 
- The lab guidance page: [Lab guidance](https://pdos.csail.mit.edu/6.824/labs/guidance.html) 
  - There are many great tips here!

## *Thinking*

- Debug是这门课lab耗时最多的部分，在完成Lab2A的过程中有所体会，在讲座中的一些点也引起了共鸣
  - 主要的过程是通过增加instrumentation缩小fault intervals从而找到最早引发error的代码
  - 我在之前没有用到的部分是
    - 读完整的代码，以及读test case；
    - 在有信心理解代码如何出错之前修改代码可能让它变得更差
    - 对log的形式化（如分栏和颜色）；
    - 多次运行test找到概率极低的bug；
    - 提问：群或是其他地方
- 在Lab2A过程中有共鸣的部分
  - 使用format string，让log简介清晰准确
  - log的详细程度，未来可以通过参数来调节
  - 尽量减少goroutine的数量和lock/channel使用的数量，
  - print statement对于timing的影响
    - 比如每次AppendEntries的失败都print的话对整个的timing影响极大，因此不可取
  - 不必频繁的调整timeout，按照test的基准以及论文的要求可以确定一个大致的范围
  - 要敢于试验新的方法，一个方法尝试了很久没有进展要敢于换方法
  - 记录过程，一个钟科学的探索方法























