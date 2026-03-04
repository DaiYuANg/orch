# Local Raft Multi-Process Smoke Test

This setup runs 3 local Warden processes on one machine to validate:

- raft leader write path works
- follower rejects scheduling writes (`not leader`)
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

Configs:

- `examples/local-raft/node1.yaml`
- `examples/local-raft/node2.yaml`
- `examples/local-raft/node3.yaml`

Workload used by smoke test:

- `examples/local-raft/echo.yaml`

## Run

```bash
go run ./cmd/localraft start -rebuild
```

Then run checks:

```bash
go run ./cmd/localraft check
```

Stop all processes:

```bash
go run ./cmd/localraft stop
```

You can also use Taskfile shortcuts:

```bash
task raft:start
task raft:check
task raft:status
task raft:stop
```

## Customize ports

Edit these keys in each node config:

- `http.port`
- `network.dns_listen`
- `network.ingress_http_listen`
- `raft.bind_addr`
- `raft.node_id`
- `raft.join` (leader node)

If ports change, update check command params:

```bash
go run ./cmd/localraft check \
  -leader-api http://127.0.0.1:7443 \
  -follower-api http://127.0.0.1:7444 \
  -follower-ingress http://127.0.0.1:18082
```
