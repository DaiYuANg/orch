# Quick Start

## Prerequisites

- Go 1.22+
- Docker (for docker runtime)
- containerd with CRI plugin and CNI config (optional, Linux-only containerd runtime)
- Firecracker binary, KVM, kernel image, rootfs image, and optional pre-created tap/bridge for firecracker runtime
- Local executables on the host (for process runtime)
- systemd on Linux or Windows Service Control Manager on Windows for native OS-service runtimes

## Build and test

```bash
go mod tidy
go test ./...
```

## Run CLI (parse deploy YAML)

```bash
go run ./cmd/orch-cli parse --file path/to/app.yaml --json
```

## Deploy and wait

```bash
go run ./cmd/orch-cli apply --file path/to/app.yaml --watch
```

## Inspect cluster state

```bash
go run ./cmd/orch-cli get workloads
go run ./cmd/orch-cli get assignments
```

Human output uses styled terminal tables. Add `--json` to keep automation output
stable.

## Run local Docker smoke test

```powershell
task smoke:local-docker
```

This starts a single-node server, deploys `examples/local-docker-smoke.yaml`,
checks the workload status with the CLI, and cleans up by default. See
`docs/local-docker-smoke.md`.

## Full-stack application example

For a fuller short `.orch` DSL example with frontend, backend, Postgres, Redis, and
ingress routing, see `examples/fullstack-docker.orch` and
`docs/fullstack-docker.md`.

For a Firecracker microVM example with a pre-created TAP device, see
`examples/firecracker-tap.orch`.

## Run server

```bash
go run ./cmd/orch-server
```

## Configuration

Environment variables use the **`ORCH`** prefix (via configx). Nested keys use double underscores, for example:

- `ORCH_HTTP__ADDR` — HTTP API bind address (default `:17443`)
- `ORCH_INGRESS_ENABLED`, `ORCH_INGRESS_ADDR`, `ORCH_INGRESS_LISTEN`

See `internal/config` and `AGENTS.md` for conventions.
