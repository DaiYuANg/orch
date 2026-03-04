# Warden

Warden is a lightweight runtime and control layer for long-lived services (database, message queue, object storage, and other always-on workloads) outside traditional container orchestration systems.

## Current Status (March 2026)

Warden is currently an MVP with a Docker-first deployment path and modular runtime architecture.

Implemented:

- Modular server bootstrapped by `fx` modules (`config`, `auth`, `task`, `registry`, `dns`, `ingress`, `http`, etc.)
- DSL parsing and validation (`yaml` and `hcl`) for workload deployment
- Task deployment API and CLI (`deploy`, `list`, `get`, `stop`, `logs`)
- Docker runtime execution, health checks, restart/reconcile loop, and managed container recovery
- Registry persistence with `bbolt` + route/endpoint resolution
- Built-in ingress (HTTP/TCP/UDP) backed by registry routes
- DNS resolution for registered services
- JWT middleware with local root token generation
- Persistent signing key storage for stable token validation across restarts
- `pack` CLI basic catalog commands (`list`, `search`)

Not finished yet:

- Production-ready non-docker runtime scheduling path (systemd/containerd/firecracker/windows-service)
- Secrets management
- HA migration/orchestration workflows
- Full dashboard experience
- Pack ecosystem and distribution pipeline

See [ROADMAP.md](/D:/Projects/warden/ROADMAP.md) for detailed plan.

## Quick Start

Requirements:

- Go
- Docker

Run server:

```powershell
go run ./cmd/server server
```

Use CLI:

```powershell
go run ./cmd/server --help
go run ./cmd/server service --help
```

Deploy sample workload:

```powershell
go run ./cmd/server service deploy --file ./examples/minimal-nginx.yaml
```

More runnable steps:

- [Deploy guide](/D:/Projects/warden/docs/content/docs/deploy.md)
- [DSL specification](/D:/Projects/warden/docs/content/docs/specification.md)

## Repository Layout

- `cmd/server`: main control-plane CLI and server entrypoint
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
go build -o ./dist/pack ./cmd/pack
```

Test:

```powershell
go test ./...
```

## License

MIT © 2025 Warden Authors
