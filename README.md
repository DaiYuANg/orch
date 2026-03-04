# Warden

Warden is a lightweight runtime and control layer for long-lived services (database, message queue, object storage, and other always-on workloads) outside traditional container orchestration systems.

## Current Status (March 2026)

Warden is currently an MVP with a modular runtime architecture and a production-focused control plane baseline.

Implemented:

- Modular server bootstrapped by `fx` modules (`config`, `auth`, `task`, `registry`, `dns`, `ingress`, `http`, etc.)
- DSL parsing and validation (`yaml` and `hcl`) for workload deployment
- Task deployment API and CLI (`deploy`, `list`, `get`, `stop`, `logs`)
- Runtime execution abstraction with driver resolver across `docker` / `containerd` / `systemd` / `firecracker` / `windows-service` (platform/runtime-gated)
- Docker runtime health checks, restart/reconcile loop, and managed container recovery
- Registry persistence with `bbolt` + route/endpoint resolution
- Raft-backed registry write path (when enabled) with FSM apply/snapshot/restore
- Leader-only scheduling guard for deploy operations with replicated assignment records
- Leader keeps worker capability enabled (leader-as-worker) and can dispatch workload runtime execution to configured remote workers (`raft.node_api`)
- Badger-backed hot cache in raft FSM for recently written consensus data
- Cluster status and membership management APIs (`/system/cluster`, `/system/cluster/join`, `/system/cluster/remove`)
- Placement control APIs for migration/failover/rebalance (`/tasks/{id}/migrate`, `/tasks/failover`, `/tasks/rebalance`)
- Stateful migration safety guardrails (`task.stateful`, `force_stateful`, `max_unavailable=1`)
- Process split between server runtime (`cmd/server`) and user operations CLI (`cmd/cli`)
- Built-in ingress (HTTP/TCP/UDP) backed by registry routes
- DNS resolution for registered services with deploy/stop driven DNS record lifecycle updates
- JWT middleware with local root token generation
- Persistent signing key storage for stable token validation across restarts
- Baseline automated tests for `internal/task`, `internal/registry`, and `internal/ingress`
- Local raft multi-process smoke test (`task raft:smoke`) for deploy+scheduling+dns+ingress validation
- Dashboard shell rebuilt with Refine + shadcn-style component primitives
- `pack` CLI basic catalog commands (`list`, `search`)

Not finished yet:

- Runtime-specific parity gaps (for example containerd logs, systemd/firecracker/windows-service deep lifecycle semantics)
- Containerd logs and full recovery parity with docker runtime
- Stateful migration/failover/rebalance strategy hardening (drain, pre-check, rollback, disruption budgets)
- Secrets management
- HA migration/orchestration workflows
- Full dashboard feature coverage (auth, deploy form, logs/actions, richer observability views)
- Pack ecosystem and distribution pipeline

See [ROADMAP.md](/D:/Projects/warden/ROADMAP.md) for detailed plan.

## Quick Start

Requirements:

- Go
- Docker

Run server:

```powershell
go run ./cmd/server run
```

Use CLI:

```powershell
go run ./cmd/cli --help
go run ./cmd/cli service --help
```

Deploy sample workload:

```powershell
go run ./cmd/cli service deploy --file ./examples/minimal-nginx.yaml
```

More runnable steps:

- [Deploy guide](/D:/Projects/warden/docs/content/docs/deploy.md)
- [DSL specification](/D:/Projects/warden/docs/content/docs/specification.md)
- [Local Raft multi-process smoke test](/D:/Projects/warden/docs/local-raft.md)

## Repository Layout

- `cmd/server`: server process entrypoint (`run` command)
- `cmd/cli`: user CLI for deploy/list/get/stop/logs/token/info/cluster operations
- `cmd/pack`: pack discovery CLI (MVP)
- `internal/task`: deployment orchestration, reconcile, health/restart
- `internal/registry`: route and endpoint registry
- `internal/dns`: DNS server using registry backends
- `internal/ingress`: HTTP/TCP/UDP ingress proxying
- `internal/runtime_engine`: runtime driver implementations
- `internal/dsl`: workload schema, parser, validator
- `dashboard`: frontend console (early stage)

## Development

Build:

```powershell
go build -o ./dist/server ./cmd/server
go build -o ./dist/warden-cli ./cmd/cli
go build -o ./dist/pack ./cmd/pack
```

Test:

```powershell
go test ./...
```

## License

MIT © 2025 Warden Authors
