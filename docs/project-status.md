# Project Status

Snapshot date: May 14, 2026

Implemented:

- Runtime abstraction with providers for `docker`, `containerd`, Linux
  `firecracker`, local `process`, Linux `systemd`, and Windows
  `windows-service`.
- The `containerd` provider targets the CRI sandbox path by default, including
  workload DNS injection through CRI sandbox DNS config.
- Workload DNS is platform-managed for container runtimes: providers inject
  orch DNS, and orch DNS can forward non-orch names to configured workload
  upstream resolvers.
- Runtime-neutral deploy spec: `run.artifact` for images/paths/URLs,
  `run.exec` for command/args, and typed `runtimeOptions` for `docker`,
  `containerd`, `firecracker`, `process`, `systemd`, and `windows-service`.
- Task APIs: deploy/list, app start/stop/restart/delete, and baseline
  migrate/failover/rebalance operations.
- Control-plane readiness API and CLI gate (`/api/ready`, `orch ready --wait`)
  backed by Raft leader discovery and write-routing checks.
- dix lifecycle diagnostics are enabled for the server: recent framework events
  are retained in memory, startup logs include build/start durations, and
  build/start/stop/provider/resolve/lifecycle-hook durations are exported
  through the existing observability backend.
- Runtime-local workload inspection API/CLI for supported providers
  (`describe workload`, `logs`), with Docker/process/Firecracker status/logs and
  fallback status for providers that have not implemented runtime-local status.
- CLI operation helpers for `wait app`, recent assignment `events`, and
  `describe node`.
- Registry-backed ingress route and endpoint management.
- DNS record lifecycle tied to deploy/stop.
- Dragonboat-backed Raft write-path coordination with TCP transport, static multi-node
  bootstrap, local status visibility, basic add/remove voter membership
  operations, and follower forwarding for deploy lifecycle and membership writes
  when `cluster.nodes` maps leader IDs to API URLs.
- Transitional DSL flow with `plan/render/apply/delete`, with the canonical
  Workload DSL v1 direction documented separately.
- DSL/compiler pipeline direction (`parser -> HIR -> binder -> IR -> canonical ->
  planner`) tracked in design docs (`docs/dsl*.md`).
- Canonical DSL subset support for `workload`, `volume`, `config`, `secret`,
  `ingress`, `import`, typed refs, `env`, `resources`, and basic HTTP health
  probes.
- Ingress served by `github.com/arcgolabs/vale` runtime/proxy through
  `internal/ingress`; longer-term snapshot / reconciliation designs are
  described in `docs/ingress*.md`.
- The repository is currently lint-clean under
  `golangci-lint run ./... --allow-serial-runners`, with tests moved toward
  external package boundaries where practical.
- The Taskfile now exposes `release-gate:static` and full `release-gate`
  workflows for beta release verification.
- Ingress runtime snapshots now carry explicit backend binding identity
  (`workload_id` / `endpoint_name`) in addition to concrete backend addresses.
- The transitional `/dsl/apply` flow now compiles explicit ingress route specs
  and reconciles route/DNS records after workload deploy, instead of relying
  only on deploy-time route side effects.
- The canonical planner output now also exposes an explicit apply object for
  ingress route specs, so the newer DSL/compiler path has a stable control-plane
  handoff shape.

In progress:

- Beta release gate hardening across lint, tests, release snapshot, Raft smoke,
  Docker lifecycle smoke, workload DNS smoke, and worker dispatch smoke.
- Runtime parity hardening (especially containerd CRI status/recovery
  semantics).
- Provider parity hardening for `systemd` and `windows-service` (status,
  recovery, and service wrapper ergonomics).
- Firecracker provider parity hardening (automatic TAP/bridge management,
  jailer integration, recovery, and image/rootfs preparation workflow).
- Stateful placement/migration policy refinement beyond the current explicit
  migrate/failover/rebalance baseline.
- More production-grade failure handling and recovery behavior.
- Higher-level Raft membership guardrails.
