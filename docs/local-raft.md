# Local Raft Cluster

orch uses `github.com/hashicorp/raft` for replicated control-plane state. The
server now uses a TCP Raft transport by default.

## Single node

The default config starts a single node on `127.0.0.1:7444`:

```bash
orch-server
```

When `raft.bind` is set to `0.0.0.0:7444` or `:7444`, also set
`raft.advertise` to the concrete `host:port` other peers can dial.

## Static multi-node bootstrap

For a new cluster, every server should start with the same `raft.peers` voter
set and a unique `raft.node.id`. Use separate data directories per node.

Node A:

```yaml
http:
  addr: "127.0.0.1:17443"
raft:
  node:
    id: node-a
  bind: "127.0.0.1:7444"
  advertise: "127.0.0.1:7444"
  peers:
    node-a: "127.0.0.1:7444"
    node-b: "127.0.0.1:7445"
    node-c: "127.0.0.1:7446"
```

Node B and C use the same `raft.peers` map, different `http.addr`,
`raft.node.id`, `raft.bind`, `raft.advertise`, and data paths.

Equivalent flags:

```bash
orch-server --raft-node-id node-a --raft-bind 127.0.0.1:7444 --raft-advertise 127.0.0.1:7444 --raft-peers node-a=127.0.0.1:7444,node-b=127.0.0.1:7445,node-c=127.0.0.1:7446
```

## Dynamic membership

Start an initial node normally. Start a new node with `raft.bootstrap: false`
so it does not create a separate one-node cluster when its Raft data directory
is empty:

```yaml
raft:
  node:
    id: node-d
  bind: "10.0.0.14:7444"
  advertise: "10.0.0.14:7444"
  bootstrap: false
```

Then target the current leader:

```bash
orch raft status
orch raft members
orch raft add-voter node-d 10.0.0.14:7444 --server http://10.0.0.11:17443
orch raft remove-voter node-d --server http://10.0.0.11:17443
```

Membership writes must be sent to the Raft leader. `orch raft status` shows the
local Raft state, known leader ID/address, local Raft address, and member count.
Automatic forwarding is still future work.

## Current limits

Static bootstrap and basic add/remove voter operations are supported. Leader
visibility is available through `orch raft status`. Automatic forwarding,
non-voter learners, and joint operational guardrails are still future work.
