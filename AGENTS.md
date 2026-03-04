# AGENTS.md — Warden

Warden is a lightweight runtime + control layer for long-lived, stateful services (DB/MQ/Object Storage, etc.) running outside of traditional container orchestrators.

This file defines how an agent should work in this repo: how to build/run/test, where to implement changes, and project conventions.

---

## Core goals

- Manage **stateful, long-lived workloads** with minimal operational overhead.
- Support **multiple runtimes/executors** (docker/containerd/systemd/firecracker/windows-service, etc.).
- Provide **service discovery** (DNS / mDNS), health, ingress, and an API layer.
- Keep the architecture **modular and evolvable** (fx modules; explicit boundaries).

Non-goals (for now):
- Full Kubernetes replacement.
- Heavy operator/controller patterns for stateful HA (may be added later, but keep core lean).

---

## Repo layout (high-level)

- `cmd/`
  - `cmd/server/`: main server CLI (Cobra). Entry: `cmd/server/main.go`
  - `cmd/pack/`: packaging/search CLI (WIP)
  - `cmd/access_client/`: access client (WIP)
- `internal/` (core)
  - `config/`: config loading via `toolkit4go/configx` (dotenv + defaults + env override + files)
  - `dsl/`: workload/unit/task definitions and parsing (HCL + YAML tags)
  - `runtime_engine/`: executors/drivers (docker/containerd/systemd/firecracker/windows service)
  - `raft/`: raft manager / replicated state plumbing
  - `registry/`: registry repository (bbolt + raft repository abstraction)
  - `dns/`, `mdns/`: discovery
  - `http/`, `endpoint/`, `ingress/`: API + endpoint routing + ingress
  - `task/`, `schedule/`: task & scheduling
  - `auth/`: JWT auth / token issuing
  - `uds/`: unix domain socket client/server helpers
- `pkg/`: reusable exported packages
- `dashboard/`: UI console
- `docs/`: design notes / docs

---

## How the server boots (important)

`warden server` (Cobra) creates an `fx.App` with modules:

- `internal/config`
- `internal/auth`
- `internal/mdns`
- `internal/raft`
- `internal/registry`
- `internal/common`
- `internal/task`
- `internal/endpoint`
- `internal/http`
- `internal/ingress`
- `internal/dns`

If you add a new subsystem, implement it as an **fx module** under `internal/<name>/module.go`, then wire it in `cmd/server/serverCmd.go`.

---

## Build / run / test

This repo uses Taskfile.

### Build (current platform)
- Server:
  - `task build:server`
  - output: `dist/server`
- Pack:
  - `task build:pack`
  - output: `dist/pack`

### Run (developer)
- Quick run:
  - `go run ./cmd/server server`
- With config files (later overrides earlier):
  - `go run ./cmd/server server --conf config.yaml --conf config.local.yaml`

### Checks / lint
- `task check` → staticcheck
- `task lint` → golangci-lint

### Tests
- `go test ./...`

Notes:
- Config loader supports `.env`, defaults, env override, and optional config files.

---

## Local development: minimal runnable config (template)

Create `config.yaml` at repo root (or pass via `--conf`).

> This template aims for: server boots + HTTP API up + DNS/mdns optional.
> Depending on which subsystems are mandatory at startup, you may need to adjust ports/paths.

```yaml
app:
  name: warden
  env: dev

log:
  level: info

http:
  enabled: true
  listen: "0.0.0.0:8080"
  # cors / tls / etc can be added later

auth:
  enabled: true
  # For dev only. Replace in prod.
  jwt:
    issuer: "warden-dev"
    audience: "warden"
    secret: "dev-secret-change-me"
    expire: "24h"

registry:
  # local store path for dev
  data_dir: "./data/registry"

raft:
  enabled: false
  # enable later for multi-node; keep off for local dev minimal boot
  # node_id: "node-1"
  # bind: "0.0.0.0:9000"
  # data_dir: "./data/raft"
  # peers: []

dns:
  enabled: false
  # listen: "0.0.0.0:5353"
  # domain: "warden.local"

mdns:
  enabled: true
  # service: "_warden._tcp"
  # port: 8080

runtime_engine:
  # choose one or more drivers enabled in dev.
  drivers:
    docker:
      enabled: true
      # docker_host: "unix:///var/run/docker.sock"
    systemd:
      enabled: false
    containerd:
      enabled: false
    firecracker:
      enabled: false

# Optional: workload examples may live under ./workloads
workload:
  dir: "./workloads"
```
# Suggested dev commands

First run:

mkdir -p data/registry workloads

go run ./cmd/server server --conf config.yaml

If your config loader supports .env:

create .env and set overrides using prefix WARDEN_...

# Contribution guidelines
## Branching

main is always releasable.

Work on feature branches:

feat/<topic>

fix/<topic>

chore/<topic>

docs/<topic>

## Commit message (Conventional Commits)

Use:

feat: ... new capability

fix: ... bug fix

refactor: ... non-functional refactor

perf: ... performance improvement

test: ... tests only

docs: ... docs only

chore: ... tooling/CI/build

Prefer small commits with clear intent.

## PR / change request expectations

A PR should include:

What changed + why (problem statement)

How to test (exact commands)

Risks / roll-back notes if behavior changes

Any new config keys + defaults

## Release / versioning

If you use goreleaser: ensure changelog-worthy commits use feat/fix/perf.

Do not break config compatibility without a migration note in docs/.

## CI expectations (local before pushing)

go test ./...

task check

task lint

Configuration conventions

Config struct: internal/config/config.go

Defaults are defined in defaultConfig().

Load path:

CLI: --conf accepts multiple files, later overrides earlier

If no --conf, it looks for: config.yaml|yml|toml|json and mock/mock.* if present

Env prefix: WARDEN_ (see internal/constant), env overrides are allowed.

When adding new config:

Add fields to internal/config/config.go with koanf tags.

Extend defaults in defaultConfig().

Avoid breaking changes: prefer additive config.

# Coding conventions (must follow)
## Dependency injection / modularity

Prefer fx.Module + fx.Provide with explicit constructors.

Avoid hidden singletons; avoid package-level mutable state.

Keep cross-module dependencies explicit and minimal.

## Error handling

Return errors with context (fmt.Errorf("...: %w", err)).

Do not swallow errors in core orchestration paths.

## Logging

Use slog where already adopted; keep logs structured (key/value).

Avoid noisy logs in hot paths unless behind debug level.

## API / endpoints

Keep handlers thin: validate → call service → map response.

Prefer small interfaces for services to simplify testing.

## DSL

Any DSL change must update:

definition structs / tags

normalization logic (if needed)

validation (if present)

docs/examples (if exist in docs/)

## Runtime/executor drivers

Changes must preserve behavior across runtimes; do not hardcode docker-only assumptions.

If you add a new driver:

isolate it under internal/runtime_engine/<driver>/

add an interface + registration point (do not leak driver types across the codebase)

# Adopt samber/lo and samber/mo (required for new code where it improves clarity)

This repo may use:

github.com/samber/lo for collection/functional helpers (map/filter/group/reduce, etc.)

github.com/samber/mo for Option[T] / Result[T] to reduce nil/err boilerplate

# Rules of use (avoid abuse)

Use lo/mo to simplify local transformations and optional values.

Do NOT turn simple readable loops into unreadable pipelines.

Avoid lo.Must / panic-style helpers in production paths.

For hot paths, prefer simple loops if profiling indicates overhead.

# Option patterns (mo.Option[T])

Use Option to represent "may be missing" without nil footguns.

Prefer:

- mo.Some(v) / mo.None[T]() (or equivalent)

- opt.OrElse(defaultValue) / opt.OrEmpty() patterns

- opt.Match(someFn, noneFn) to keep branching explicit

Guidelines:

Input parsing / config: return Option for optional fields

Domain model: avoid *T when optional semantics matter; use Option[T] (unless JSON tags force pointers)

# Result patterns (mo.Result[T])

Use Result when you frequently need: value + error with map/flatMap chains.

Prefer:

- mo.Ok(v) / mo.Err[T](err)

- res.Map(fn) / res.FlatMap(fn) for pipelines

- res.Match(okFn, errFn) when branching

## lo patterns

Use lo for:

`lo.Map`, `lo.Filter`, `lo.Reduce`, `lo.GroupBy`

`lo.Associate`, `lo.KeyBy`, `lo.UniqBy`

Avoid:

deep nesting of lambdas

heavy allocations inside tight loops

Migration principle

New code should consider lo/mo first.

Existing code may be gradually refactored when touched, but avoid churn-only PRs.

# How to implement common changes
## Add a new server subsystem

1. Create internal/<name>/module.go with fx.Module("<name>", fx.Provide(...))

2. Add services/structs under internal/<name>/

3. Wire module in cmd/server/serverCmd.go modules = []fx.Option{ ... }

4. Add tests for the core logic (not necessarily fx wiring)

## Add a new CLI command

- Add file under cmd/server/<xxx>Cmd.go

- Register it in cmd/server/rootCmd.go commands = []*cobra.Command{ ... }

- Keep CLI commands as composition roots; business logic should live under internal/

## Review checklist for agents

Before finishing a change:

- go test ./... passes

- task check and task lint pass (or explain why not)

- No new cyclic dependencies across internal/*

- Config changes are additive + defaults updated

- Any public behavior change has docs note under docs/ or README.md

## When unsure

If requirements are unclear:

- Prefer minimal, additive changes.

- Leave TODOs only with concrete next steps and file pointers.

- Do not invent API contracts—search existing internal/http, internal/endpoint, internal/dsl patterns first.