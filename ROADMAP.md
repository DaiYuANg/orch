# Warden Roadmap

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
- [x] CLI operations for service deploy/list/get/stop/logs
- [x] JWT auth middleware and root token generation
- [x] Persistent auth signing key for restart-safe token validation
- [x] Baseline tests for `task` deployment lifecycle and runtime injection
- [x] Baseline tests for `registry` route resolution and owner cleanup
- [x] Baseline tests for `ingress` host normalization and TCP route sync
- [x] Dashboard shell migrated to Refine + shadcn-style UI primitives
- [x] Raft FSM command apply (`set/delete`) with snapshot/restore implementation
- [x] Raft-enabled registry mutating operations routed through consensus apply
- [x] Leader-only deploy guard with replicated scheduling-assignment records
- [x] Badger hot-cache integration for raft FSM read/write path
- [x] Leader-as-worker scheduling baseline with desired/worker assignment metadata
- [x] Cluster observability and membership APIs (`/system/cluster`, `join`, `remove`)
- [x] CLI/process split: `cmd/server` for server runtime and `cmd/cli` for user operations

## In Progress

- [ ] Runtime abstraction hardening for non-docker executors (systemd/containerd/firecracker/windows-service)
- [ ] Cross-node remote execution dispatch after leader placement (current scheduling baseline is leader-local)
- [ ] Dashboard integration depth (auth/token flow, deploy actions, logs and richer runtime operations)
- [ ] Pack CLI and package workflow beyond static catalog
- [ ] Better operator UX for auth/token/config management
- [ ] More automated tests in `cmd/*`, plus deeper edge-case coverage in runtime/ingress flows

## Planned

### Runtime and orchestration

- [ ] First-class support for systemd/containerd/firecracker/windows-service execution flows
- [ ] Placement and migration strategies for stateful workloads
- [ ] HA failover primitives and explicit rebalancing operations

### Platform capabilities

- [ ] Secrets injection and secret source integrations
- [ ] Pluggable health checks and lifecycle hooks
- [ ] Multi-node coordination model refinement (raft/gossip responsibilities)
- [ ] Fine-grained API authn/authz and token lifecycle management

### Developer and user experience

- [ ] Stable dashboard release with operations views and deployment actions
- [ ] Pack format, metadata, and remote registry/distribution model
- [ ] Operational docs for single-node and multi-node production setups
- [ ] Compatibility matrix and release quality gates
