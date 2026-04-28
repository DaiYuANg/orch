# orch Roadmap

Snapshot date: March 4, 2026

## Completed

- [x] `fx` modular server composition and lifecycle wiring
- [x] Config loading via defaults + env + optional config files
- [x] Workload DSL parsing/validation (`yaml` + `hcl`)
- [x] Docker-first deployment pipeline (`/tasks/deploy`)
- [x] Deployment lifecycle APIs (`list`, `get`, `stop`, instance `logs`)
- [x] Registry persistence for endpoints/routes (`bbolt`)
- [x] DNS resolution from service registry
- [x] Built-in ingress for HTTP/TCP/UDP route forwarding
- [x] Reconcile loop: container restart and managed container recovery
- [x] Task runtime interface abstraction with Docker adapter
- [x] Driver-based runtime resolver baseline with containerd adapter wiring
- [x] Task runtime factory wired for `docker/containerd/systemd/firecracker/windows-service` with platform-aware executors/stubs
- [x] CLI operations for service deploy/list/get/stop/logs
- [x] JWT auth middleware and root token generation
- [x] Persistent auth signing key for restart-safe token validation
- [x] Baseline tests for `task` deployment lifecycle and runtime injection
- [x] Baseline tests for `registry` route resolution and owner cleanup
- [x] Baseline tests for `ingress` host normalization and TCP route sync
- [x] Frontend packages removed from this repository (dashboard extracted to standalone project)
- [x] Documentation stack migrated to mdBook (`docs/`)
- [x] Raft FSM command apply (`set/delete`) with snapshot/restore implementation
- [x] Raft-enabled registry mutating operations routed through consensus apply
- [x] Leader-only deploy guard with replicated scheduling-assignment records
- [x] Badger hot-cache integration for raft FSM read/write path
- [x] Leader-as-worker scheduling baseline with desired/worker assignment metadata
- [x] Cross-node runtime dispatch baseline via worker API (`raft.node_api`) with leader fallback
- [x] Placement control baseline via deployment migration API (`/tasks/{id}/migrate`)
- [x] Explicit failover/rebalance control baselines (`/tasks/failover`, `/tasks/rebalance`)
- [x] Stateful migration safety guardrail baseline (`task.stateful`, `force_stateful`, `max_unavailable=1`)
- [x] Cluster observability and membership APIs (`/system/cluster`, `join`, `remove`)
- [x] CLI/process split: `cmd/server` for server runtime and `cmd/cli` for user operations
- [x] Deploy/stop DNS record lifecycle binding for HTTP ingress hosts
- [x] Local raft smoke test harness (`task raft:smoke` + `tests/localraft`)

## In Progress

- [ ] Runtime abstraction hardening for non-docker executors (runtime-specific lifecycle parity and operational semantics)
- [ ] Containerd parity gaps (logs, managed-recovery parity, richer network semantics)
- [ ] Cross-node reconcile/restart/log aggregation path for remote worker instances (baseline wired, needs production hardening)
- [ ] Stateful-safe migration/failover/rebalance policies beyond guardrails (drain, pre-check, rollback, disruption budgets)
- [ ] Pack CLI and package workflow beyond static catalog
- [ ] Better operator UX for auth/token/config management
- [ ] More automated tests in `cmd/*`, plus deeper edge-case coverage in runtime/ingress flows

## Planned

### Runtime and orchestration

- [ ] First-class support for systemd/containerd/firecracker/windows-service execution flows
- [ ] Advanced placement and migration strategies for stateful workloads
- [ ] HA failover and explicit rebalancing with policy-driven automation

### Platform capabilities

- [ ] Secrets injection and secret source integrations
- [ ] Pluggable health checks and lifecycle hooks
- [ ] Multi-node coordination model refinement (raft/gossip responsibilities)
- [ ] Fine-grained API authn/authz and token lifecycle management

### Developer and user experience

- [ ] External dashboard project stabilization and integration boundary docs
- [ ] Pack format, metadata, and remote registry/distribution model
- [ ] Operational docs for single-node and multi-node production setups
- [ ] Compatibility matrix and release quality gates
