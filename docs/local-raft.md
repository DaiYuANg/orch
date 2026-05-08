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

Then send the membership write to any node whose config has `cluster.nodes`
mapping the current leader ID to its HTTP API URL:

```bash
orch raft status
orch raft members
orch raft add-voter node-d 10.0.0.14:7444 --server http://10.0.0.11:17443
orch raft remove-voter node-d --server http://10.0.0.11:17443
```

If the contacted node is a follower, it forwards deploy lifecycle and operation
writes (`apply`, `start`, `stop`, `restart`, `delete`, `migrate`, `failover`,
`rebalance`) plus Raft membership writes to the known leader. `orch raft status`
shows the local Raft state, known leader ID/address, leader API URL when
configured, local Raft address, and member count.

## Local forwarding smoke

Run a local three-node Raft cluster and prove that write requests submitted to a
follower are forwarded to the leader and replicated:

```powershell
task smoke:local-raft-forwarding
```

The smoke starts three `orch-server` processes with static peer bootstrap,
applies `examples/local-raft-forwarding.yaml` through a follower, waits for the
app to appear on every node, then deletes it through a follower and waits for
the deletion to replicate.

## Current limits

Static bootstrap and basic add/remove voter operations are supported. Leader
visibility is available through `orch raft status`. Follower forwarding depends
on `cluster.nodes` containing the leader's HTTP URL. Non-voter learners and
joint operational guardrails are still future work.
