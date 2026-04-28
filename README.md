# orch

orch is a lightweight runtime and control layer for long-lived services (database, message queue, object storage, and other always-on workloads) outside traditional container orchestration systems.

## Status

orch is being built in Go and focuses on:

- Multi-runtime workload execution (`docker`, `containerd`, `firecracker`)
- Built-in ingress and DNS record lifecycle
- Raft-aware scheduling (deploy/migrate/failover/rebalance)
- Declarative Workload DSL direction with compatibility inputs and `plan/render/apply/delete`

Current core stack:

- DI: `github.com/arcgolabs/dix`
- Logging: `github.com/arcgolabs/logx`
- HTTP server/API: `github.com/arcgolabs/httpx` (`fiber` adapter)
- Ingress: embedded `github.com/caddyserver/caddy/v2`
- CLI: `github.com/spf13/cobra`
- Consensus path: `github.com/hashicorp/raft`

## Project Structure

```text
cmd/
  orch-cli/              # cobra entrypoint
  orch-server/           # process entrypoint + graceful shutdown
internal/
  config/                # env-based config
  logging/               # logx factory
  security/auth/         # authx guard module
  httpserver/            # httpx bootstrap (adapter/fiber + openapi)
  api/                   # typed route registration
  deploy/v1alpha1/       # canonical deploy YAML model
  runtime/
    docker/              # docker provider (v1)
    containerd/          # containerd provider (v1)
  services/
    registry/            # workload registry
    task/                # deploy orchestration service
  raftsvc/               # hashicorp/raft service boundary
```

## Quick Start

### Parse a deploy YAML file

```bash
go run ./cmd/orch-cli dsl parse --file path/to/app.yaml --json
```

### Run server (skeleton)

```bash
go run ./cmd/orch-server
```

## Documentation (mdBook)

Docs are now maintained with `mdBook`.

```bash
mdbook serve docs
```

See:

- [Introduction](docs/introduction.md)
- [Project Status](docs/project-status.md)
- [Quick Start](docs/quick-start.md)
- [Workload DSL v1 (EN)](docs/dsl.md)
- [Workload DSL v1（中文）](docs/dsl.zh.md)
- [Ingress Design v1 (EN)](docs/ingress.md)
- [Ingress 设计 v1（中文）](docs/ingress.zh.md)
- [Local Raft Cluster](docs/local-raft.md)

## Development

```bash
go mod tidy
go test ./...
```

## Ingress Configuration

Environment variables use the **`ORCH`** prefix (see `internal/config`, loaded via configx).

- `ORCH_INGRESS_ENABLED` (default: `true`)
- Default listeners: `:80` and `:443`. Set `ORCH_INGRESS_ADDR` to use a single port instead (for example `:8088` on machines where binding low ports requires elevated privileges).

## License

MIT
