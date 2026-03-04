# Project Status

Snapshot date: March 4, 2026

Implemented:

- Runtime abstraction with providers for `docker`, `containerd`, and `firecracker`.
- Task APIs: deploy, stop, migrate, failover, rebalance.
- Registry-backed ingress route and endpoint management.
- DNS record lifecycle tied to deploy/stop.
- Raft-aware write-path coordination baseline.
- Local multi-process cluster helpers via `cargo xtask`.
- Declarative DSL v1 with `plan/render/apply/delete`.

In progress:

- Runtime parity hardening (especially deeper containerd semantics).
- Stateful placement/migration policy refinement.
- More production-grade failure handling and recovery behavior.
