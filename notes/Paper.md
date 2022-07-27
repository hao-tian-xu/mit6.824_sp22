# MapReduce: Simplified Data Processing on Large Clusters

<img src="image.assets/Screen Shot 2022-06-14 at 12.56.44.png" alt="Screen Shot 2022-06-14 at 12.56.44" style="zoom: 50%;" />

1. **Introduction**

2. **Programming Model**

   1. Example

   2. Types

   3. More Examples:

      Distributed Grep, Count of URL Access Frequency, Reverse Web-Link Graph, Term-Vector per Host, Inverted Index, Distributed Sort

3. **Implementation**

   1. Execution Overview

   2. Master Data Structures

   3. Fault Tolerance

      Worker Failure, Master Failure, Semantics in the Presence of Failures

   4. Locality

   5. Task Granularity

   6. Backup Tasks

4. **Refinement**

   1. Partitioning Function
   2. Ordering Guarantees
   3. Combiner Function
   4. Input and Output Types
   5. Side-effects
   6. Skipping Bad Records
   7. Local Execution
   8. Status Information
   9. Counters

5. **Performance**

   1. Cluster Configuration
   2. Grep
   3. Sort
   4. Effect of Backup Tasks
   5. Machine Failures

6. **Experience**

   1. Large-Scale Indexing

7. **Related Work**

8. **Conclusions**

### *Thinking*

- 这篇论文介绍了Google在2003年开发的一个分布式计算框架MapReduce
  - 一个关键的意义是让不熟悉分布式系统的程序员也可以使用，也因为框架提供了简单的接口，使得Google的indexing代码变得易读和易修改
  - 介绍了分布式系统的一些基本要求如何得到满足，如Fault Tolerance，Consistency等
  - 使用Backup Tasks来解决计算的最后阶段一些计算机运行过慢的问题（straggler）



# The Google File System

<img src="image.assets/Screen Shot 2022-06-21 at 15.31.26.png" alt="Screen Shot 2022-06-21 at 15.31.26" style="zoom: 50%;" />

1. **Introduction**
2. **Design Overview**
   1. Assumptions
   2. Interface
   3. Architecture
   4. Single Master
   5. Chunk Size
   6. Metadata
      1. In-Memory Data Structures
      2. Chunk Locations
      3. Operation Log
   7. Consistency Model
      1. Guarantees by GFS
      2. Implications for Applications
3. **System Interactions**
   1. Leases and Mutation Order
      - ​	<img src="image.assets/Screen Shot 2022-06-21 at 15.30.53.png" alt="Screen Shot 2022-06-21 at 15.30.53" style="zoom: 33%;" />
   2. Data Flow
   3. Atomic Record Appends
   4. Snapshot
4. **Master Operations**
   1. Namespace Management and Locking
   2. Replica Placement
   3. Creation, Re-replication, Rebalancing
   4. Garbage Collection
      1. Mechanism
      2. Discussion
   5. Stale Replica Detection
5. **Fault Tolerance and Diagnosis**
   1. High Availability
      1. Fast Revovery
      2. Chunk Replication
      3. Master Replication
   2. Data Integrity
   3. Diagnostic Tools
6. **Measurements**
   1. Micro-benchmarks
      1. Reads
      2. Writes
      3. Record Appends
   2. Real World Clusters
      1. Storage
      2. Metadata
      3. Read and Write Rates
      4. Master Load
      5. Recovery Time
   3. Workload Breakdown
      1. Methodology and Caveats
      2. Chunkserver Workload
      3. Appends versus Writes
      4. Master Workload
7. **Experiences**
8. **Related Work**
9. **Conclusions**

### *Thinking*

- 这篇2003年的论文介绍了Google File System，一个分布式存储系统
  - 介绍了一些符合实际应用需求的设计决策
    - 如将文件系统管理和数据传输分离，前者通过primary到secondaries，而后者直接在各server间传递（避免树状传递浪费网络带宽）



# The Design of a Practical System for Fault-Tolerant Virtual Machines

1. **INTRODUCTION**
2. **BASIC FT DESIGN**
   1. Deterministic Replay Implementation
   2. FT Protocal
   3. Detecting and Responding to Failure
3. **PRACTICAL IMPLEMENTATION OF FT**
   1. Starting and Restarting FT VMs
   2. Managing the Logging Channel
   3. Operations on FT VMS
   4. Implementation Issues for Disk IOs
   5. Implementation Issues for Network IO
4. **DESIGN ALTERNATIVES**
   1. Shared vs. Non-shared Disk
   2. Executing Disk Reads on the Backup VM
5. **PERFORMANCE EVALUATION**
   1. Basic Performance Results
   2. Network Benchmarks
6. **RELATED WORK**
7. **CONCLUSION AND FUTURE WORK**

### *Thinking*

- 介绍了VMware开发的一个基于虚拟机的容错（fault-tolerant）系统
  - 核心是传输一系列的决定性的指令，而非数据
    - 非决定性指令转化为决定性的指令（基于虚拟机）
  - 一些设计决定的讨论
    - 硬盘读操作传输数据
    - 使用同一个共享文件系统



# In Search of an Understandable Consensus Algorithm (Extended Version)

1. **Introduction**
   - understandability: decomposition and state space reduction
2. **Replicated state machines**
   - consensus algorithm: keep the replicated log consistent
3. **What's wrong with Pxos**
4. **Designing for understandability**
5. **The Raft concensus algorithm**
   1. Raft basics
   2. Leader election
      - understandability -> randomized retry than a ranking system (subtle corner cases)
   3. Log replication
   4. Safety
      1. Election restriction
      2. Committing entries from previous terms
         - To eliminate problems like the one in Figure 8, Raft never commits log entries from previous terms by count- ing replicas.
      3. Safety argument
   5. Follower and candidate crashes
   6. Timing and availability
6. **Cluster membership changes**
7. **Log compaction**
8. **Client interaction**
9. **Implementation and evaluation**
   1. Understandability
   2. Correctness
   3. Performance
10. **Related work**
11. **Conclusion**

### *Thinking-1 (to section 5)*

- 这篇论文介绍了一个共识算法Raft
  - 主要机制是Majority Election，保证了一个commited log entry出现在未来的所有leader的log中，因此保证了所有server的state machine所应用的同一编号的log entry所包含的内容是相同的，即state machine safety
  - Raft一个关键的设计特征是可理解性，因此在关键的leader election的过程中，在一次选举失败后，采用了随机重试的方法（避免各种corner case）
    - 随机和概率（当代性）替代了确定性

### *Thinking-2 (section 7 to end)*

- 两个研究/论文的方法
  - 由于raft算法设计的目的是understandability，如何对这个相对主观的标准进行对比评价
  - 使用TLA+ specification language来做出正式的specification，并由该formal spec证明其correctness



# ZooKeeper: Wait-free coordination for Internet-scale systems

1. Introduction
2. The ZooKeeper service
   1. Service overview
   2. Client API
   3. ZooKeeper guarantees
   4. Examples of primitives
3. ZooKeeper Appliations
4. ZooKeeper Implementation
   1. Request Processor
   2. Atomic Broadcast
   3. Replicated Database
   4. Client-Server Interactions
5. Evaluation
   1. Throughput
   2. Latency of requests
   3. Performance of barriers
6. Related work
7. Conclusions

### *Thinking*

- 介绍了一个基于Zab（类似Raft）的无等待协调服务，由于对象是互联网规模的系统，因此性能是关键因素
  - 提供了两个顺序的保障
    - 一个是写操作是linearizable的（但读操作不是，因此性能和服务器数量是线性相关的）
    - 另一个是单个client的操作是按顺序的
  - 通过znode的类型以及版本号让需要strong consistency的client可以自行实现mini-transaction或lock等操作
    - 由于对象是互联网规模的系统，也提出了client避免herd effect的方法



# Chain Replication for Supporting High Throughput and Availability

1. Introduction
2. A Storage Service Interface
3. Chain Replication Protocol
   1. Protocol Details
4. Primary/Backup Protocols
5. Simulation Experiments
   1. Single Chain No Failures
   2. Multiple Chains, No Failures
   3. Effects of Failures on Throughput
   4. Large Scale Replication of Critical Data
6. Related Work
7. Concluding Remarks

### *Thinking*

- 介绍了链式复制：一个可以同时支持高吞吐量和Linearizability的分布式存储
  - 通过链式复制write操作产生的状态改变，以及所有reply都由tail发出保证了Linearizability
  - 通过将数据分布在不同的链中，从而读操作由不同的tail完成，保证了Scalability和高吞吐量



# Frangipani: A Scalable Distributed File System

1. Introduction
2. System Structure
   1. Components
   2. Security and the Client/Server Configuration
   3. Discussion
3. Disk Layout
4. Logging and Recovery
5. Synchronization and Cache Coherence
6. The Lock Service
7. Adding and Removing Servers
8. Backup
9. Performance
   1. Experimental Setup
   2. Single Machine Performance
   3. Scaling
   4. Effects of Lock Contention
10. Related Work
11. Conclusions

### *Thinking*

- 介绍了Frangipani，一个分布式文件系统，该系统首次提出了几个重要的概念
  - Cache Coherence：保证不同用户在（使用本地缓存）读写同一文件的strong consistency
    - 前设条件是大部分人都主要在自己的文件上工作，因此文件操作在本地缓存中进行
  - Distributed Locking：通过lock servers和busy、idle、write-ahead logging等机制，提供了strong consistency的同时保证了performance
    - 类似unix系统中硬盘的写操作
  - Distributed Crash Recovery：一个server崩溃后可由系统中任意server进行恢复（类似unix系统中的硬盘恢复）



# Spanner: Google’s Globally-Distributed Database

1. Introduction
2. Implementation
   1. Spanserver Software Stack
   2. Directories and Placement
   3. Data Model
3. True Time
4. Concurrency Control
   1. Timestamp Mangagement
      1. Paxos Leader Leases
      2. Assigning Timestamps to RW Transactions
      3. Serving Reads at a Timestamp
      4. Assigning Timestamps to RO Transactions
   2. Details
      1. Read-Write Transactions
      2. Read-Only Transactions
      3. Schema-Change Transactions
      4. Refinements
5. Evaluation
   1. Microbenchmarks
   2. Availability
   3. TrueTime
   4. F1
6. Related Work
7. Future Work
8. Conclusions

































