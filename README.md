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
- I don't want to send a message more than once, if a node gets a duplicate we should avoid passing it on to the neighbors.
- The state of a node should be locked when it is read or changed to avoid inconsistencies.

I implemented this by creating the three handlers called for by the challenge, `broadcast`, `read`, and `topology`, plus a fourth handler called `gossip` which does the same as `broadcast` (store and gossip the message if it is new), except it doesn't wait for a reply. This way the node replies to Maelstrom when it gets a `broadcast` message, but within the cluster `gossip` uses a fire-and-forget, cutting the number of messages required in half.

Any time the nodes messages or neighbors are read or updated locks are used to maintain consistency.
