# Warden

Warden is a lightweight runtime and control layer for long-lived services (database, message queue, object storage, and other always-on workloads) outside traditional container orchestration systems.

## Status

Warden is now Rust-first and focuses on:

- Multi-runtime workload execution (`docker`, `containerd`, `firecracker`)
- Built-in ingress and DNS record lifecycle
- Raft-aware scheduling (deploy/migrate/failover/rebalance)
- Declarative Workload DSL direction with compatibility inputs and `plan/render/apply/delete`

## Quick Start

### Run server

```bash
cargo run -p warden-server -- --conf examples/local-raft/node1.yaml
```

### Query workloads

```bash
cargo run -p warden-cli -- --api http://127.0.0.1:7443 workloads
```

### Read task logs

```bash
cargo run -p warden-cli -- --api auto task logs <workload-id> --tail 200
```

### Use Current DSL Flow

The repository currently has two DSL-related paths:

- The older transitional manifest-oriented `dsl plan/apply/delete` flow
- The newer canonical compiler pipeline inspectable through `dsl planner`

The current implemented canonical subset, supported syntax, and current limits
are tracked in `docs/src/dsl.md`.

```bash
cargo run -p warden-cli -- --api http://127.0.0.1:7443 dsl plan --file examples/dsl-v1-demo.yaml --json
cargo run -p warden-cli -- --api http://127.0.0.1:7443 dsl apply --file examples/dsl-v1-demo.yaml --prune --concurrency 8
cargo run -p warden-cli -- dsl planner --file path/to/app.wd
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
- [Workload DSL v1 (EN)](docs/src/dsl.md)
- [Workload DSL v1（中文）](docs/src/dsl.zh.md)
- [Ingress Design v1 (EN)](docs/src/ingress.md)
- [Ingress 设计 v1（中文）](docs/src/ingress.zh.md)
- [Local Raft Cluster](docs/src/local-raft.md)

## Development

```bash
cargo fmt --all
cargo check --workspace
cargo test --workspace
```

## License

MIT
