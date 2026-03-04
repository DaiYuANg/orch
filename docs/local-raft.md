# Local Raft Multi-Process Smoke Test

This setup runs 3 to 4 local Warden processes on one machine to validate:

- raft leader write path works
- follower rejects scheduling writes (`not leader`)
- leader can dispatch runtime execution to remote worker nodes
- follower ingress can still route traffic after replicated registry updates

## Ports and nodes

- node1:
  - api: `7443`
  - raft: `12001`
  - dns: `10531`
  - ingress: `18081`
- node2:
  - api: `7444`
  - raft: `12002`
  - dns: `10532`
  - ingress: `18082`
- node3:
  - api: `7445`
  - raft: `12003`
  - dns: `10533`
  - ingress: `18083`
- node4 (optional):
  - api: `7446`
  - raft: `12004`
  - dns: `10534`
  - ingress: `18084`

Configs:

- `examples/local-raft/node1.yaml`
- `examples/local-raft/node2.yaml`
- `examples/local-raft/node3.yaml`
- `examples/local-raft/node4.yaml` (optional)

Workload used by smoke test:

- `examples/local-raft/echo.yaml`

## Run

Open 3 terminals and run one node per terminal.
Each node runs in foreground, so logs are printed directly in that terminal.

Terminal A (start follower first):

```bash
task raft:node2
```

Terminal B (start follower first):

```bash
task raft:node3
```

Terminal C (start bootstrap node last):

```bash
task raft:node1
```

Optional Terminal D:

```bash
task raft:node4
```

Stop nodes with `Ctrl+C` in each terminal.

## Basic validation

Deploy workload to leader (node1):

```bash
task raft:deploy:leader
```

Try deploying the same workload to follower (node2), should fail with `not leader`:

```bash
task raft:deploy:follower
```

Read deployment list from leader and follower:

```bash
task raft:list:leader
task raft:list:follower
```

Check raft status from leader and follower:

```bash
task raft:cluster:leader
task raft:cluster:follower
```

Run one-command smoke validation (nodes must already be running):

```bash
task raft:smoke
```

## Customize ports

Edit these keys in each node config:

- `http.port`
- `network.dns_listen`
- `network.ingress_http_listen`
- `raft.bind_addr`
- `raft.node_id`
- `raft.api_addr`
- `raft.node_api`
- `raft.join` (leader node)

`raft.node_api` format uses `node-id=http://api-host:port`, for example:

```yaml
raft:
  node_api:
    - "127.0.0.1:12001=http://127.0.0.1:7443"
    - "127.0.0.1:12002=http://127.0.0.1:7444"
```

If ports change, update config files and Task vars.
For example, deploy to another leader API:

```bash
task raft:deploy:leader API=http://127.0.0.1:7443
```
