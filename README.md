# Gossip Glomers
Working through [Gossip Glomers](https://fly.io/dist-sys/), a series of distributed systems challenges by fly.io. 

## Maelstrom
> [Maelstrom](https://github.com/jepsen-io/maelstrom/tree/main) is a workbench for learning distributed systems by writing your own. It uses the Jepsen testing library to test toy implementations of distributed systems.

In Maelstrom a Node is a binary that receives JSON messages from STDIN and sends JSON messages to STDOUT. [protocol spec](https://github.com/jepsen-io/maelstrom/blob/main/doc/protocol.md)

## Setup
Download maelstrom from [jepsen-io/maelstrom_releases](https://github.com/jepsen-io/maelstrom/releases) and unpack it into ./maelstrom/ at the repo root. The justfile expects ./maelstrom/maelstrom to exist.

### macOS note

Maelstrom needs a Java runtime. If you install OpenJDK via Homebrew (`brew install openjdk`), it's keg-only and macOS's system Java wrapper won't find it, so running `./maelstrom` fails with:

> Unable to locate a Java Runtime.

Symlink it into the system JVM directory:

```sh
sudo ln -sfn /opt/homebrew/opt/openjdk/libexec/openjdk.jdk \
  /Library/Java/JavaVirtualMachines/openjdk.jdk
```

## [Challenge 1: Echo](https://fly.io/dist-sys/1/)
Just a quick hello world sort of thing to make sure it's all configured. 

## [Challenge 2: Unique ID Generation](https://fly.io/dist-sys/2/)
The task is to set up a node that returns a unique ID when prompted.

At first I solved this using UUIDs, which felt like the naive approach. I don't think it's bad but it does give a pretty long ID which can be annoying in some contexts. I ended up going with a concatenation of the node ID and a per-node atomic counter. Of course this relies on Maelstrom giving each node a unique ID [which it does](https://github.com/jepsen-io/maelstrom/blob/main/doc/protocol.md#nodes-and-networks). 

## Challenge 3: Broadcast
Implement a broadcast system that gossips messages between all nodes in a cluster.

### [Challenge 3a: Single-Node Broadcast](https://fly.io/dist-sys/3a/)
Part A is a trivial single-node scenario. Store messages and return them on read.

### [Challenge 3b: Multi-Node Broadcast](https://fly.io/dist-sys/3b/)
Part B is the multi-node scenario. Each message must be gossiped to all other nodes. The challenge warns against choosing naive solutions that wouldn't scale to real world problems, such as sending all the data on a node to all other nodes each time it gets a message. 

Gossiping the data more efficiently requires keeping a few things in mind:
- Maelstrom provides a topology message on each run which tells the nodes who their neighbors are. I'll use this to decide where to send messages (for now at least, custom topologies are also possible).
- I don't want to send a message more than once, if a node gets a duplicate, we should avoid passing it on to the neighbors.
- The state of a node should be locked when it is read or changed to avoid inconsistencies.

I implemented this by creating the three handlers called for by the challenge, `broadcast`, `read`, and `topology`, plus a fourth handler called `gossip` which does the same as `broadcast` (store and gossip the message if it is new), except it doesn't wait for a reply. This way the node replies to Maelstrom when it gets a `broadcast` message, but within the cluster `gossip` uses a fire-and-forget, cutting the number of messages required in half.

Any time the node's messages or neighbors are read or updated locks are used to maintain consistency.

### [Challenge 3c: Fault Tolerant Broadcast](https://fly.io/dist-sys/3c/)
#### Plan
This challenge adds network partitions meaning that some times nodes will not be able to communicate with each other. By the end of the test all messages should still propagate to all nodes. I can think of a few ways to accomplish this, all have trade-offs:
1. For each neighbor, maintain a set of messages to send, require an acknowledgement for each gossip and remove shared messages from the set when acknowledged. Send the whole unacknowledged set at a defined cadence until it's acknowledged.
  - Initially it occured to me to send all unacknowledged messages each time a new message was received, so that anything missed would be brought up to date, but this breaks down in the scenario where the last message fails. A timer-based retry until the outgoing set is empty handles this.
  - If the outstanding message set gets too large, there may be need to send it as several subsets to avoid single operations sending tons of data.
2. For each shared message, require an acknowledgement, if it doesn't come, retry sending the message until it is received. 
  - Some backoff and timeout settings would be needed to avoid network spikes or never-ending retries.
  - This approach has potential to create a lot of traffic, but all the messages would be small.
3. Just gossip all the messages a node has each time it shares data, store the union of its own data and received data.
  - This is a naive approach that won't scale to large systems, messages would eventually become huge. It'd probably solve the challenge, but I don't think it's the right approach.

Option 1 could be described as batch draining a set of pending messages. It's not quite a queue because order doesn't matter and we'll dedupe pending messages before sending. I think I'll try this approach first and see how it goes. 

#### Implementation
I solved this one by adding a `outbox` map to the node which stores a set of messages that it needs to send to each neighbor. The node expects an acknowledgement to the gossip which it uses to remove successfully gossiped messages from the outbox. This way if there is a network partition the messages will remain in the outbox and be retried on the next call of `gossip()`. To ensure that gossip is called eventually, I use a ticker to call gossip() every 100ms. 

There is a bit of room for optimization here for scenarios where a node receives the same message in multiple gossips since we only dedupe the messages for the gossiper. In other words, a node knows who it's telling, but not who other nodes might be telling, so two nodes might both tell the same neighbor the same message. It looks like the next challenges get in to this so I'll address it there.

### [Challenge 3d: Efficient Broadcast, Part 1](https://fly.io/dist-sys/3d/)
This challenge introduces performance targets. The task is to run broadcast with 25 nodes and a delay of 100ms per message (to simulate network latency) with less than 400ms median latency, less than 600ms max latency, and less than 30 messages per operation.
- **latency**: The time difference between when a broadcast was acknowledged and when it was last missing from a read on any node. I.e., how long it takes a message to propagate through the network.
- **messages-per-operation**: The number of messages exchanged per logical operation (mostly `broadcast` and `read` in a ~50/50 split). This is dominated by broadcast which can create many messages as they propagate through the network. `read` and `topology` only create a single reply. 

A few things need to change:
- I'll switch from outbox-based gossiping to keeping a list of nodes which have recieved each message with the message, and passing that around. This way a node will know all nodes who have seen the message on it's path, rather than just the last node to see it. I'll call this `knowers`. This will help the messages/operation metric. This was a significant refactor, but I think it's a better fit for this system.
- The default grid based topology won't work for the latency target. With a 5x5 grid, some messages will need 8 hops to fully propagate. That'll be around 800ms at least, and we're shooting for P1.0 under 600. 
- A star topology drives the latency way down, to just over 200ms, which makes sense because the longest path should be 2 hops. But the messages/op is still a bit over 30. It seems like it should be around 26 (50/broadcast and 2/read averages to 26) so I think we're duplicating gossips in the time between the first gossip and the ack which adds it to the knowers. To solve I configured it to optimistically adds the destination node to knowers when a message is sent, and remove it from knowers if there is an RPC error. The ticker will clean up any cases where we fail to send a message because we optimistically assumed the destination knew it. **This configuration passes 3a-d with the same code.**

### [Challenge 3e: Efficient Broadcast, Part 2](https://fly.io/dist-sys/3e/)
Same maelstrom configuration as 3d (25 nodes, 100ms message delay), but this time we're shooting for fewer than 20 messages / operation. Latency targets are relaxed to P0.5 <= 1 second, P1.0 <= 2 seconds. 

The first thing I see here is that even with a star topology, which I think is the most message efficient topology, we can't hit the target if we gossip after each broadcast and expect acknowledgments for each gossip, that would take 50 messages per broadcast and 2 per read. I see two options: don't require acks, or gossip less frequently. Intuitively, not requiring acks seems like it'll make it very difficult to be sure that we maintain consistency across the cluster. I'll explore gossiping less frequently first. Up to this point I've tried to make sure that each implementation continued to pass the preceeding broadcast challenges, I think for 3d and 3e it will be difficult to pass both with the same config, we'll need to decide between optimizing for lower traffic or lower latency as we get closer to the limits of the system. 

This one turned out to be really easy, I just commented out the calls to gossip() in `handleBroadcast` and `handleGossip` and let the ticker do it all, which got it down to two messages per op, with a max latency under 1 second and a median just under 700ms (with a 500ms ticker). I'm not going to commit these changes, but the knobs to adjust between messages/op and latency in this system are the tick rate, and the gossip triggers, and the topology. On to challenge 4.


## [Challenge 4: Grow-Only Counter](https://fly.io/dist-sys/4/)
The goal is to implement a stateless, grow-only counter. It needs to be eventually consistent. 

Had to do a bit of reading for this one, [this wiki page was helpful](https://en.wikipedia.org/wiki/Conflict-free_replicated_data_type#G-Counter_(Grow-only_Counter)).

This challenge tells us to use the SeqKV [service](https://github.com/jepsen-io/maelstrom/blob/main/doc/services.md) provided by Maelstrom. 
> Services are Maelstrom-provided nodes 

> A sequentially consistent key-value store. Just like lin-kv, but with relaxed consistency.
> All operations appear to take place in a total order. Each client observes a strictly monotonic order of operations. However, clients may interact with past states of the key-value store, provided that interaction does not violate these ordering constraints.

To keep track of the values in a network that may (will) experience partitions, we can have each node write to it's own key. When asked for a count, we sum the values from all keys. The gotcha is that we need to make sure that we're looking at the latest version of the values, not a cached version. To do this we can do a no-op write to a dummy key. To be honest, this felt like a bit of a hack, but I think this approach is the intention of the challenge given that they are asking us to get a globally current count from a sequential KV. In a real-world scenario we would probably have two different read methods, one slower, "exact" read like the one implemented here in `handleRead` and another "fast" read that didn't required globally current values (otherwise what would be the point of using SeqKV instead of LinKV?).

## Challenge 5: Kafka-Style Log
### [Challenge 5a: Single-Node Kafka-Style Log](https://fly.io/dist-sys/5a/)
This is a first step towards implementing a replicated log similar to Apache Kafka. In 5a we just make a system that works on a single node, (keeping in mind that each node can have multiple topics). In the next challenges we'll expand to multi-node and then work on improving performance. 

Since this is based on Kafka, I'll use [Kafka's design documentation](https://kafka.apache.org/43/design/design/) as a guide.

### [Challenge 5b: Multi-Node Kafka-Style Log](https://fly.io/dist-sys/5b/)
Now we have multiple nodes. The challenge allows the use of a linear KV store service provided by Maelstrom. It seems like the challenge is pointing us towards storing the data on a linear key-value store and using the nodes as stateless front-end systems that can interact with clients, and use a shared KV to maintain a global order to the entries in the log. This is different than Kafka, which stores data in the brokers and uses a leader-based approach where events are not consumable until they have reached each broker/node in the cluster (i.e., they become *committed*). It is similar to Kafka in that it's an append-only ordered log and consumers can provide an offset to consume or re-consume data from the log.

I used the Linear KV to store the logs and the committed offsets. I used a CAS loop like in the g-counter to handle sends, and a simple write for handling commit offsets (last-write-wins). It passed. There is room for improvement here, but I think that's coming in 5c since the challenge states:
> Your nodes can use the linearizable key/value store provided by Maelstrom to implement your distributed, replicated log. This challenge is about correctness and not efficiency. You only need to keep up with a reasonable request rate. It’s important to consider which components require linearizability versus sequential consistency.

I think offsets can be stored in a sequential kv since it's fine to be eventually consistent there (if we start from an offset that's older than the newest offset, that's OK, the order is still correct, a consumer might just consume the same message twice which still fits with at-least-once semantics).

### [Challenge 5c: Efficient Kafka-Style Log](https://fly.io/dist-sys/5c/)
The goal here is more open-ended than other challenges, we're instructed to reduce messages/operation, reduce latency, and increase availability. Better start with a baseline, here are the stats from a run of challenge 5b:
- msg/op: 9.1
- availability: 0.9995
- worst-realtime-lag: 31 seconds (!)

I made a few changes which dropped the lag way down at the expense of a few more messages. I think it's a more scalable system too.
- Stopped sending the whole log on each CAS.
- Improved `handleSend` by storing each message at its own key (`log/<key>/<offset>`) using CAS-create to claim a slot. This way no two senders write the same offset. The next offset is also stored as a point to start looking from. We can write optimistically to the next offset marker since the worst case is that we start a bit behind the tail and loop through the CAS retries to find the end of the log.
- Improved poll so that it can get as many messages as there are between the last written offset and the end of the log. It turns out that the "worst-realtime-lag" metric depends on the ability of `handlePoll` to return enough messages in a single reply. With the default settings for the challenge it will poll from offset 0 late in the test, requiring at least 16 messages to be returned, else it states that the lag is the entire duration of the test. I think in a real-world system we could consider paging this response to bound the size of the reply, but here I just need to make sure that my maxMessages is high enough. 

With the changes:
- msg/op: ~14 (up ~5 msg/op)
- availability: 0.9996 (same)
- worst-realtime-lag: ~0.1s (down ~30s)

There are certainly more things that could be improved but I'm going to move on. Some ideas:
- Parallel reads in handle poll
- Caching local version of the values (can be done when writing and reading). 
- Move committed offsets to a SeqKV. 


## Challenge 6: Totally-Available Transactions
### [Challenge 6a: Single-Node, Totally-Available Transactions](https://fly.io/dist-sys/6a/)
In this challenge we make our own KV store which implements transactions. A transaction is a series of reads and/or writes which either all succeed or all fail, i.e., the whole transaction happens atomically. For the first challenge we just do it on a single node. 

Kept it simple here. The server holds the kv store as a map, locked with a mutex during reads and writes.