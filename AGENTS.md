# AGENTS.md - orch

orch is a lightweight runtime and control layer for long-lived, stateful services
(DB/MQ/Object Storage, etc.) outside traditional container orchestrators.

This file defines how an agent should work in this repository: build/run/test flows,
where to implement changes, and project conventions.

---

## Core goals

- Manage stateful, long-lived workloads with low operational overhead.
- Support multiple runtimes/executors (`docker`, `containerd`, `firecracker`, future `systemd/windows-service`).
- Provide service discovery (DNS), ingress, health, API, and cluster scheduling controls.
- Keep architecture modular and evolvable via Go packages.

Non-goals for now:

- Full Kubernetes replacement.
- Heavy operator/controller patterns before core runtime parity is solid.

---

## Tech baseline (current)

- Language: Go (1.22+).
- CLI: `cobra`.
- Deploy: canonical document is `internal/deploy/v1alpha1.App`. Compatibility layers are separate packages: `internal/deploy/orch` (.orch DSL), `internal/deploy/composeimport` (Docker Compose), `internal/deploy/loader` (manifest dispatch: .orch vs YAML).
- Raft: `github.com/hashicorp/raft` integration path.
- Tooling:
  - `task` (go-task) for daily developer commands.
  - `mdBook` for docs (`docs/`).
  - Releases: `.goreleaser.yaml` + `task release-snapshot` / `task goreleaser-check` (archives + deb/rpm/apk via nfpm).
  - Dev Container: `.devcontainer/` (Go 1.26 bookworm, `task`, Delve, Docker CLI via docker-outside-of-docker host socket; forwards `17443`).

---

## Repo layout (high-level)

- `cmd/`
 - `cmd/orch-server/`: server binary entrypoint.
 - `cmd/orch-cli/`: CLI entrypoint (`cmd/orch-cli/cmd` — cobra commands; `cmd/orch-cli/cliapp` — orch-cli-only dix composition, not domain libraries).
 - `cmd/orch-vpn/`: workstation daemon for orch-vpn; builds a short-lived dix graph (`internal/orchvpn` WorkstationApp) similar to orch-cli cluster commands.
- `pkg/`
 - `pkg/oopsx`: stable helper API ([samber/oops](https://github.com/samber/oops) wrappers); safe for external modules to import.
- `internal/`
 - `internal/deploy/v1alpha1`: canonical deploy model (types, validation, YAML load/parse).
 - `internal/deploy/orch`: `.orch` plano DSL compatibility ([orch.Module] → `*orch.Orch`).
 - `internal/deploy/composeimport`: Docker Compose → v1alpha1 import.
 - `internal/deploy/loader`: manifest [Loader] (.orch via orch, YAML via v1alpha1); register `loader.Module` after `orch.Module`.
 - `internal/runtime/*`: runtime abstraction and providers (docker/containerd first).
 - `internal/api/*`: HTTP API layer (OpenAPI via httpx/fiber).
 - `internal/services/*`: registry, task orchestration.
 - `internal/dnssvc`, `internal/ingress`, `internal/orchvpn`, `internal/raftsvc`, `internal/scheduler`: control-plane services and orch-vpn tunnel (in progress).
- `docs/`: mdBook sources (`docs/book.toml`, `docs/src/*`).

Note: Frontend dashboard source has been removed from this repository and is maintained externally.

---

## Boot path (important)

`orch-server` startup flow in Go:

1. Parse config (flags/env/files via configx).
2. Load validated config.
3. Initialize logger.
4. Wire DI modules (registry, DNS, ingress, runtime, raft, task, scheduler, HTTP API).
5. Register runtime providers (`docker`, `containerd`, …).
6. Start DNS, ingress, raft, scheduler, HTTP transport.
7. Optional: reachable-endpoints logging.

When adding a new subsystem, wire it explicitly in this composition root (`cmd/orch-server/main.go` and `internal/*/module.go`).

---

## Build, run, test

Primary workflow uses `task` (go-task):

- `task tidy` -> `go mod tidy`
- `task test` -> `go test ./...`
- `task run-cli -- <args>` / `task run-server -- <args>`

Docs:

- `mdbook build docs`
- `mdbook serve docs`

---

## Local development quick start

CLI (parse deploy YAML):

```bash
go run ./cmd/orch-cli parse --file path/to/app.yaml --json
```

Server:

```bash
go run ./cmd/orch-server
```

---

## Config conventions

Source of truth: `internal/config` + `internal/deploy/v1alpha1` (orch/composeimport/loader translate into this model).

- Keep config changes additive.
- Provide sensible defaults in `Default()` / typed defaults.
- Validate before runtime uses config where applicable.
- Supported file formats: `.yaml/.yml/.toml/.json`.
- Env overrides:
  - `ORCH__...` (preferred nested form).
  - `ORCH_...` (flat form).

When adding config:

1. Add field in config struct.
2. Add default value.
3. Add validation where needed.
4. Update docs/examples if behavior changes.

---

## API and transport conventions

- Handlers should stay thin: parse/validate -> service call -> response mapping.
- Keep business logic in `internal/services/*` and domain packages, not in handlers.
- OpenAPI/Swagger is exposed via httpx + fiber (`/openapi.json`, `/swagger-ui`).

HTTP defaults:

- Control plane listens on `:17443` unless overridden (`HTTP.Addr` / `ORCH_HTTP__ADDR`).

---

## Runtime and scheduling conventions

- `internal/runtime` is the abstraction boundary; drivers live under `internal/runtime/<driver>/`.
- Do not leak driver concrete types into API-facing structs.
- Scheduling/orchestration belongs in `internal/services/task` and future scheduler pipelines.
- Mutating cluster state that requires consensus should go through Raft apply paths when wired.

When adding a runtime driver:

1. Add `internal/runtime/<driver>/`.
2. Implement `runtime.Provider`.
3. Register in `internal/runtime/module.go`.
4. Add focused tests for lifecycle parity (deploy/stop/logs/recovery basics).

---

## CLI conventions

- CLI is a composition layer in `cmd/orch-cli`.
- Command definitions use `cobra`.
- Subcommands live under `cmd/orch-cli/cmd`.
- Control-plane commands compose a short-lived `dix` graph in `cmd/orch-cli/cliapp`: cluster-facing commands use `RunCluster` (`Conn` → `*apiclient.Client` + `loader.Loader` + `OnStop` close); manifest-only commands use `RunManifest` (`orch.Module` + `loader.Module`, no HTTP client).
- Keep output stable JSON for automation-friendly usage.

When adding a new command:

1. Add cobra args/subcommand definitions.
2. Wire handlers in `cmd/orch-cli/cmd`.
3. Reuse shared packages; do not duplicate transport logic (prefer `cmd/orch-cli/cliapp` + `internal/apiclient`).
4. Add/update docs examples.

---

## Coding standards

- No hidden global mutable state.
- Return contextual errors with wrapping (`fmt.Errorf("...: %w", err)`).
- Use structured logs with clear fields.
- Prefer small modules and explicit boundaries.
- Keep hot-path code simple; avoid unnecessary allocations/abstractions.

Go conventions:

- Keep packages cohesive; avoid cyclic deps.
- Keep `cmd/*` as composition roots; put logic in `internal/*`.

---

## Contribution and commit conventions

Branch naming:

- `feat/<topic>`
- `fix/<topic>`
- `chore/<topic>`
- `docs/<topic>`

Commit messages: Conventional Commits (`feat:`, `fix:`, `refactor:`, `perf:`, `test:`, `docs:`, `chore:`).

Before finishing changes:

- `gofmt -w .`
- `go test ./...`
- Update docs/README/ROADMAP when behavior changes.

---

## When unsure

- Prefer minimal, additive changes.
- Do not invent API contracts; inspect existing packages first.
- Leave TODOs only with concrete next steps and file pointers.
- If design is unclear, align with existing package boundaries rather than introducing a new architecture.
