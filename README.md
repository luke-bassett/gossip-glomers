Download maelstrom from [jepsen-io/maelstrom_releases]() and unpack it into ./maelstrom/ at the repo root. The justfile expects ./maelstrom/maelstrom to exist.


In Maelstrom a Node is a binary that receives JSON messages from STDIN and sends JSON messages to STDOUT. [protocol spec](https://github.com/jepsen-io/maelstrom/blob/main/doc/protocol.md)

## macOS setup note

Maelstrom needs a Java runtime. If you install OpenJDK via Homebrew (`brew install openjdk`), it's keg-only and macOS's system Java wrapper won't find it, so running `./maelstrom` fails with:

> Unable to locate a Java Runtime.

Symlink it into the system JVM directory:

```sh
sudo ln -sfn /opt/homebrew/opt/openjdk/libexec/openjdk.jdk \
  /Library/Java/JavaVirtualMachines/openjdk.jdk
```

## Challenge 2: Unique ID Generation
At first I solved this using UUIDs, which felt like the naive approach. I don't think it's bad but it does give a pretty long id which can be annoying in some contexts. I ended up going with a concatenation of the node id and a per-node atomic counter. Of course this relies on Maelstrom giving each node a unique ID. [TODO: can I rely on maelstrom giving nodes unique ids?]
