# Warden

Warden is a lightweight runtime and control layer for long-lived services (database, message queue, object storage, and other always-on workloads) outside traditional container orchestration systems.

## Status

Warden is now Rust-first and focuses on:

- Multi-runtime workload execution (`docker`, `containerd`, `firecracker`)
- Built-in ingress and DNS record lifecycle
- Raft-aware scheduling (deploy/migrate/failover/rebalance)
- Declarative DSL v1 (`Application` manifest with `plan/render/apply/delete`)

## Quick Start

### Run server

```bash
cargo run -p warden-server-rs -- --conf examples/local-raft/node1.yaml
```

### Query workloads

```bash
cargo run -p warden-cli-rs -- --api http://127.0.0.1:7443 workloads
```

### Read task logs

```bash
cargo run -p warden-cli-rs -- --api auto task logs <workload-id> --tail 200
```

### Use DSL v1

```bash
cargo run -p warden-cli-rs -- --api http://127.0.0.1:7443 dsl plan --file examples/dsl-v1-demo.yaml --json
cargo run -p warden-cli-rs -- --api http://127.0.0.1:7443 dsl apply --file examples/dsl-v1-demo.yaml --prune --concurrency 8
```

## Documentation (mdBook)

Docs are now maintained with `mdBook`.

```bash
mdbook serve docs
```

See:

- [Introduction](docs/src/introduction.md)
- [Project Status](docs/src/project-status.md)
- [Quick Start](docs/src/quick-start.md)
- [DSL v1](docs/src/dsl-v1.md)
- [Local Raft Cluster](docs/src/local-raft.md)

## Development

```bash
cargo fmt --all
cargo check --workspace
cargo test --workspace
```

## License

MIT
