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
