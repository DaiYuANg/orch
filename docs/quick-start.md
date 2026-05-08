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
go run ./cmd/orch-cli get apps
go run ./cmd/orch-cli describe app my-app -n default
```

Human output uses styled terminal tables. Add `--json` to keep automation output
stable. App status includes desired and observed generations; if an assignment
has not reconciled the current desired generation yet, that workload is reported
as pending.

## Start/stop/delete an app

```bash
go run ./cmd/orch-cli stop app my-app -n default
go run ./cmd/orch-cli start app my-app -n default
go run ./cmd/orch-cli restart app my-app -n default
go run ./cmd/orch-cli delete app my-app -n default
```

`stop` stops assigned workloads through the local runtime or worker dispatch and
keeps the desired app document. `start` uses that retained desired state to run
the app again, `restart` does stop then start, and `delete` stops workloads first
then removes desired state from Raft.

## Host DNS installer hook

Linux packages run the host DNS installer during package install/removal. For
manual development checks:

```bash
go run ./cmd/orch-server host-dns status --json
go run ./cmd/orch-server host-dns install
```

This configures the OS resolver for the orch DNS zone. Workloads remain unaware
of the host resolver setup.

## Raft membership

```bash
go run ./cmd/orch-cli raft status
go run ./cmd/orch-cli raft members
go run ./cmd/orch-cli raft add-voter node-b 10.0.0.12:7444
go run ./cmd/orch-cli raft remove-voter node-b
```

Membership writes must target the current Raft leader. Use `raft status` to
check local state and the known leader. For a node that will be joined
dynamically, start it with `raft.bootstrap: false`.

## Run local Docker smoke test

```powershell
task smoke:local-docker
task smoke:local-docker-dns
task smoke:local-docker-worker-dispatch
```

This starts a single-node server, deploys `examples/local-docker-smoke.yaml`,
checks workload status with the CLI, runs stop/start/restart/delete, and cleans
up by default. The DNS smoke deploys two workloads and verifies the client can
reach `dns-backend.default.svc.orch.local` through orch DNS. The worker dispatch
smoke starts separate scheduler and worker server processes and verifies remote
dispatch through the worker API. See `docs/local-docker-smoke.md`,
`docs/local-docker-dns-smoke.md`, and
`docs/local-docker-worker-dispatch-smoke.md`.

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
