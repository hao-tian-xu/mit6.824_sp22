# Fault-tolerant Sharded Key/Value Service based on Raft

(Labs of MIT 6.824: Distributed Systems)



Labs 2-4 implement a sharded key/value service in three steps:

1. Replicated state machines following the [extended Raft paper](https://pdos.csail.mit.edu/6.824/papers/raft-extended.pdf). 
2. A fault-tolerant key/value service using the Raft implementation. 
3. A shard controller and sharded key/value servers both use a similar framework of the simple key/value service in step 2.



The first lab is to build a MapReduce system similar to the [MapReduce paper](http://research.google.com/archive/mapreduce-osdi04.pdf).

