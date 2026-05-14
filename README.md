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
At first I solved this using UUIDs, which felt like the naive approach. I don't think it's bad but it does give a pretty long ID which can be annoying in some contexts. I ended up going with a concatenation of the node ID and a per-node atomic counter. Of course this relies on Maelstrom giving each node a unique ID [which it does](https://github.com/jepsen-io/maelstrom/blob/main/doc/protocol.md#nodes-and-networks). 

## [Challenge 3: Broadcast](https://fly.io/dist-sys/3a/)
### Parts A and B
In part A we only have a single node, so there are no other nodes to broadcast to. I jumped right to part B and set up the topology-based formatting, and deduplication to avoid loops. This setup passes the tests for both part A and part B.
