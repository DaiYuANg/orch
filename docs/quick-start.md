# Quick Start

## Prerequisites

- Go 1.22+
- Docker (for docker runtime)
- containerd (optional, for containerd runtime later)

## Build and test

```bash
go mod tidy
go test ./...
```

## Run CLI (parse deploy YAML)

```bash
go run ./cmd/orch-cli dsl parse --file path/to/app.yaml --json
```

## Run server

```bash
go run ./cmd/orch-server
```

## Configuration

Environment variables use the **`ORCH`** prefix (via configx). Nested keys use double underscores, for example:

- `ORCH_HTTP__ADDR` — HTTP API bind address (default `:17443`)
- `ORCH_INGRESS_ENABLED`, `ORCH_INGRESS_ADDR`, `ORCH_INGRESS_LISTEN`

See `internal/config` and `AGENTS.md` for conventions.
