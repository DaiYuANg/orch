# Project Status

Snapshot date: May 7, 2026

Implemented:

- Runtime abstraction with providers for `docker`, `containerd`, Linux
  `firecracker`, local `process`, Linux `systemd`, and Windows
  `windows-service`.
- Runtime-neutral deploy spec: `run.artifact` for images/paths/URLs,
  `run.exec` for command/args, and typed `runtimeOptions` for `docker`,
  `containerd`, `firecracker`, `process`, `systemd`, and `windows-service`.
- Task APIs: deploy, stop, migrate, failover, rebalance.
- Registry-backed ingress route and endpoint management.
- DNS record lifecycle tied to deploy/stop.
- Raft-aware write-path coordination baseline.
- Transitional DSL flow with `plan/render/apply/delete`, with the canonical
  Workload DSL v1 direction documented separately.
- DSL/compiler pipeline direction (`parser -> HIR -> binder -> IR -> canonical ->
  planner`) tracked in design docs (`docs/dsl*.md`).
- Canonical DSL subset support for `workload`, `volume`, `config`, `secret`,
  `ingress`, `import`, typed refs, `env`, `resources`, and basic HTTP health
  probes.
- Ingress served by embedded Caddy (`internal/ingress`); longer-term snapshot /
  reconciliation designs are described in `docs/ingress*.md`.
- Ingress runtime snapshots now carry explicit backend binding identity
  (`workload_id` / `endpoint_name`) in addition to concrete backend addresses.
- The transitional `/dsl/apply` flow now compiles explicit ingress route specs
  and reconciles route/DNS records after workload deploy, instead of relying
  only on deploy-time route side effects.
- The canonical planner output now also exposes an explicit apply object for
  ingress route specs, so the newer DSL/compiler path has a stable control-plane
  handoff shape.

In progress:

- Runtime parity hardening (especially deeper containerd semantics).
- Provider parity hardening for `systemd` and `windows-service` (status,
  recovery, and service wrapper ergonomics).
- Firecracker provider parity hardening (automatic TAP/bridge management,
  jailer integration, recovery, and image/rootfs preparation workflow).
- Stateful placement/migration policy refinement.
- More production-grade failure handling and recovery behavior.
