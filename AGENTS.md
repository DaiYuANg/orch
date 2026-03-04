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
- Keep architecture modular and evolvable via Rust workspace crates.

Non-goals for now:

- Full Kubernetes replacement.
- Heavy operator/controller patterns before core runtime parity is solid.

---

## Tech baseline (current)

- Language: Rust (workspace, edition 2024).
- API: `axum` + `utoipa` + Swagger UI.
- Config: `figment` (defaults + config files + env overrides).
- Storage: `redb` backend through `warden-store` abstraction.
- Runtime adapters: `warden-runtime-*` crates.
- Raft: `openraft` integration path.
- Tooling:
  - `just` for daily developer commands.
  - `cargo xtask` for complex orchestration (cluster/e2e/package).
  - `mdBook` for docs (`docs/`).

---

## Repo layout (high-level)

- `apps/`
  - `apps/warden-server-rs/`: server binary entrypoint.
  - `apps/warden-cli-rs/`: user CLI entrypoint.
- `crates/`
  - `warden-api`: HTTP routing, handlers, OpenAPI registration.
  - `warden-client`: CLI/API client transport (`auto`, `unix://`, `npipe://`, `http(s)://`).
  - `warden-config`: config structs + loader + validation.
  - `warden-http`: listener runtime (TCP + UDS on unix + named pipe proxy on windows).
  - `warden-runtime`: runtime abstraction and provider registry.
  - `warden-runtime-docker`: Docker runtime provider.
  - `warden-runtime-containerd`: containerd runtime provider.
  - `warden-runtime-firecracker`: Firecracker runtime provider.
  - `warden-task`: deploy/stop/migrate/failover/rebalance scheduling logic.
  - `warden-dns`: DNS record service.
  - `warden-ingress`: ingress routing/proxy service.
  - `warden-registry`: registry service over store.
  - `warden-raft`: raft service abstraction (`openraft`-backed path).
  - `warden-store`: persistent state abstraction/backend.
  - `warden-types`: shared API/domain types.
  - `warden-dsl`: DSL parse/plan/apply support.
  - `warden-logger`: tracing/log bootstrap.
- `xtask/`: `cargo xtask` command implementations.
- `examples/`: local configs and DSL examples.
- `docs/`: mdBook sources (`docs/book.toml`, `docs/src/*`).

Note: Frontend dashboard source has been removed from this repository and is maintained externally.

---

## Boot path (important)

`warden-server-rs` startup flow in `apps/warden-server-rs/src/main.rs`:

1. Parse `--conf` arguments.
2. Load validated config via `warden-config`.
3. Initialize logger.
4. Build store and seed demo baseline data.
5. Create registry, DNS, ingress, runtime engine, raft service, task service.
6. Register runtime providers (`docker`, `containerd`, `firecracker`).
7. Start DNS, ingress, task, raft services.
8. Build API router and run HTTP transport listeners.

When adding a new subsystem, wire it explicitly in this composition root.

---

## Build, run, test

Primary workflow uses `just`:

- `just check` -> `cargo check --workspace`
- `just fmt` / `just fmt-check`
- `just lint` -> `cargo clippy --workspace --all-targets -- -D warnings`
- `just test` -> `cargo test --workspace`
- `just run --conf examples/local-raft/node1.yaml`

Direct cargo equivalents are acceptable:

- `cargo check --workspace`
- `cargo fmt --all`
- `cargo clippy --workspace --all-targets -- -D warnings`
- `cargo test --workspace`

Complex workflows use `cargo xtask`:

- `cargo xtask cluster run --nodes 3 --start-port 7443`
- `cargo xtask cluster status`
- `cargo xtask cluster stop`
- `cargo xtask e2e ...`
- `cargo xtask package`

Docs:

- `just docs-build` or `mdbook build docs`
- `just docs-serve` or `mdbook serve docs`

---

## Local development quick start

Single node:

```bash
cargo run -p warden-server-rs -- --conf examples/local-raft/node1.yaml
```

CLI query:

```bash
cargo run -p warden-cli-rs -- --api auto workloads
```

Local multi-node simulation:

```bash
cargo xtask cluster run --nodes 4 --start-port 7443
```

Use `examples/local-raft/node*.yaml` for explicit per-node config.

---

## Config conventions

Source of truth: `crates/warden-config/src/lib.rs`.

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
  - unix: `unix://...` then `http://127.0.0.1:7443`
  - windows: `npipe://...` then `http://127.0.0.1:7443`
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

- CLI is a composition layer in `apps/warden-cli-rs`.
- Command definitions live in `cli_args.rs`.
- DSL-specific command logic lives in `dsl_cmd.rs`.
- Keep output stable JSON for automation-friendly usage.

When adding a new command:

1. Add clap args/subcommand definitions.
2. Add client call path in `main.rs` (or extracted command module).
3. Reuse `warden-client`; do not duplicate transport logic.
4. Add/update docs examples.

---

## Coding standards

- No hidden global mutable state.
- Return contextual errors with `anyhow::Context`.
- Use structured logs (`tracing`) with clear targets and fields.
- Prefer small modules and explicit boundaries.
- Keep hot-path code simple; avoid unnecessary allocations/abstractions.

No Go-era conventions apply anymore:

- Do not introduce new Go code.
- Do not add `fx` module wiring patterns.
- Do not reintroduce Taskfile as primary workflow.

---

## Contribution and commit conventions

Branch naming:

- `feat/<topic>`
- `fix/<topic>`
- `chore/<topic>`
- `docs/<topic>`

Commit messages: Conventional Commits (`feat:`, `fix:`, `refactor:`, `perf:`, `test:`, `docs:`, `chore:`).

Before finishing changes:

- `cargo fmt --all`
- `cargo check --workspace`
- `cargo test --workspace`
- `cargo clippy --workspace --all-targets -- -D warnings` (or explain why not)
- Update docs/README/ROADMAP when behavior changes.

---

## When unsure

- Prefer minimal, additive changes.
- Do not invent API contracts; inspect existing crates first.
- Leave TODOs only with concrete next steps and file pointers.
- If design is unclear, align with existing crate boundaries rather than introducing a new architecture.
