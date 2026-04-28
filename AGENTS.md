# AGENTS.md - Warden

Warden is a lightweight runtime and control layer for long-lived, stateful services
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
- Deploy file: YAML (canonical model in `internal/deploy/v1alpha1`).
- Raft: `github.com/hashicorp/raft` integration path.
- Tooling:
  - `task` (go-task) for daily developer commands.
  - `mdBook` for docs (`docs/`).

---

## Repo layout (high-level)

- `cmd/`
 - `cmd/orch-server/`: server binary entrypoint.
 - `cmd/orch-cli/`: CLI entrypoint.
- `internal/`
 - `internal/deploy/v1alpha1`: canonical deploy YAML model (v0.1).
 - `internal/runtime/*`: runtime abstraction and providers (docker/containerd first).
 - `internal/api/*`: HTTP API layer (planned).
- `docs/`: mdBook sources (`docs/book.toml`, `docs/src/*`).
- `docs/`: mdBook sources (`docs/book.toml`, `docs/src/*`).

Note: Frontend dashboard source has been removed from this repository and is maintained externally.

---

## Boot path (important)

`orch-server` startup flow in Go (planned):

1. Parse `--conf` arguments.
2. Load validated config.
3. Initialize logger.
4. Build store and seed demo baseline data.
5. Create registry, DNS, ingress, runtime engine, raft service, task service.
6. Register runtime providers (`docker`, `containerd`, `firecracker`).
7. Start DNS, ingress, task, raft services.
8. Build API router and run HTTP transport listeners.

When adding a new subsystem, wire it explicitly in this composition root.

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
go run ./cmd/orch-cli dsl parse --file path/to/app.yaml --json
```

Server (skeleton):

```bash
go run ./cmd/orch-server
```

---

## Config conventions

Source of truth (Go rewrite): `internal/deploy/v1alpha1` + future `internal/config`.

- Keep config changes additive.
- Provide sensible defaults in `impl Default for Config`.
- Validate with `validator` rules before runtime uses config.
- Supported file formats: `.yaml/.yml/.toml/.json`.
- Env overrides:
  - `WARDEN__...` (preferred nested form).
  - `WARDEN_...` (compatibility form).

When adding config:

1. Add field in config struct.
2. Add default value.
3. Add validation where needed.
4. Update docs/examples if behavior changes.

---

## API and transport conventions

- Handlers should stay thin: parse/validate -> service call -> envelope mapping.
- Keep business logic in service crates (`warden-task`, `warden-registry`, etc.), not in handler functions.
- Keep OpenAPI in sync:
  - Add route docs annotations in handlers.
  - Register schema/path in `crates/warden-api/src/doc.rs`.
- Swagger UI is served from `/swagger-ui`.

CLI/server transport:

- Prefer platform-local endpoints first via `--api auto`:
  - unix: `unix://...` then `http://127.0.0.1:17443`
  - windows: `npipe://...` then `http://127.0.0.1:17443`
- Explicit endpoint forms supported: `auto`, `unix://`, `npipe://`, `http://`, `https://`.

---

## Runtime and scheduling conventions

- `warden-runtime` is the abstraction boundary.
- Driver-specific logic must live in driver crates (`warden-runtime-docker`, etc.).
- Do not leak driver concrete types into API/type crates.
- Keep scheduling decisions in `warden-task`.
- Mutating operations that require consensus should go through raft apply paths.

When adding a runtime driver:

1. Add `crates/warden-runtime-<driver>/`.
2. Implement provider trait(s) from `warden-runtime`.
3. Register in server composition root.
4. Add focused tests for lifecycle parity (deploy/stop/logs/recovery basics).

---

## CLI conventions

- CLI is a composition layer in `cmd/orch-cli`.
- Command definitions use `cobra`.
- Subcommands follow Cobra template layout in `cmd/orch-cli/cmd`.
- Keep output stable JSON for automation-friendly usage.

When adding a new command:

1. Add cobra args/subcommand definitions.
2. Add command handler wiring in `cmd/orch-cli/cmd`.
3. Reuse shared packages; do not duplicate transport logic.
4. Add/update docs examples.

---

## Coding standards

- No hidden global mutable state.
- Return contextual errors with wrapping (`fmt.Errorf("...: %w", err)`).
- Use structured logs with clear fields (logger TBD).
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
- Do not invent API contracts; inspect existing crates first.
- Leave TODOs only with concrete next steps and file pointers.
- If design is unclear, align with existing crate boundaries rather than introducing a new architecture.
