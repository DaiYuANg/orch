# Gossip-Based Cluster Discovery

Snapshot date: May 14, 2026

orch can use a gossip membership layer to reduce static Raft peer
configuration. Gossip is intentionally not a consensus mechanism in orch:
Raft remains the only source of truth for replicated control-plane state and
membership changes.

## Responsibility Split

| Layer | Responsibility |
| --- | --- |
| Gossip | Discover candidate nodes and exchange node metadata such as `node_id`, `raft_addr`, `api_url`, and build version. |
| Raft | Own durable control-plane state and authoritative voter membership. |
| Scheduler | May use gossip health as a soft placement signal in future work, but should not treat gossip as durable state. |

Gossip events never directly mutate workloads, assignments, or Raft membership.
Only the current Raft leader may convert a discovered gossip node into a Raft
voter through the normal `AddVoter` path.

## Configuration

```yaml
gossip:
  enabled: true
  bind: "0.0.0.0:7946"
  advertise: "10.0.0.11:7946"
  seeds:
    - "10.0.0.10:7946"
  secret_key: "1234567890123456"
  api_url: "http://10.0.0.11:17443"
  auto_join_raft: true
  reconcile_interval: "5s"

raft:
  node:
    id: "node-a"
  bind: "0.0.0.0:7444"
  advertise: "10.0.0.11:7444"
  bootstrap: true
```

`gossip.secret_key` is optional. When set, it must be 16, 24, or 32 bytes so
memberlist can use AES-128, AES-192, or AES-256.

## Bootstrap Rule

The first cluster node may start a new single-node Raft cluster:

```yaml
raft:
  bootstrap: true
gossip:
  enabled: true
  auto_join_raft: true
```

Additional nodes should start as joiners:

```yaml
raft:
  bootstrap: false
gossip:
  enabled: true
  seeds:
    - "10.0.0.11:7946"
  auto_join_raft: true
```

This rule matters. If multiple empty nodes all start with `raft.bootstrap=true`
and cannot see each other yet, they may each create separate single-node Raft
clusters. Gossip discovery is weakly consistent, so it must not be used to
decide that a node is safe to bootstrap a brand-new consensus cluster.

## Auto-Join Flow

1. Each server starts Raft using its local config.
2. If `gossip.enabled=true`, it starts memberlist and joins configured seeds.
3. Each node publishes gossip metadata:
   - `node_id`
   - `raft_addr`
   - `api_url`
   - build version
4. The current Raft leader periodically scans alive gossip members.
5. For each alive discovered node not already in Raft membership, the leader
   calls the normal Raft `AddVoter` operation.

Dead or suspect gossip nodes are not automatically removed from Raft membership
in this first version. Removal still requires explicit Raft membership handling.

## Current Limits

- Gossip discovers Raft voters only; worker dispatch and follower forwarding
  still use `cluster.nodes` for API URL lookup.
- Automatic removal of dead Raft members is intentionally not implemented.
- Seedless bootstrap is intentionally not implemented.
- Gossip health is not yet used by scheduler placement.
