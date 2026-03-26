# Introduction

Warden is a runtime and control layer for long-lived services running outside traditional container orchestration.

Core directions:

- Keep stateful service operations lightweight.
- Provide runtime abstraction for multiple backends.
- Keep network lifecycle integrated (ingress + DNS).
- Support single-node and multi-node operation with Raft-aware scheduling.

Current implementation is Rust-first and centered around:

- `apps/warden-server`
- `apps/warden-cli`
- `crates/warden-*`
