# Quick Start

## Prerequisites

- Rust toolchain
- Docker (for docker/containerd runtime tests)

## Build and test

```bash
cargo fmt --all
cargo check --workspace
cargo test --workspace
```

## Run a local server

```bash
cargo run -p warden-server-rs -- --conf examples/local-raft/node1.yaml
```

## Basic CLI checks

```bash
cargo run -p warden-cli-rs -- --api http://127.0.0.1:7443 workloads
cargo run -p warden-cli-rs -- --api http://127.0.0.1:7443 routes
cargo run -p warden-cli-rs -- --api http://127.0.0.1:7443 dns
```

## Start local multi-node cluster

```bash
cargo xtask cluster run --nodes 3 --start-port 7443
cargo xtask cluster status
cargo xtask cluster stop
```
