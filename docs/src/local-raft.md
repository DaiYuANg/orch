# Local Raft Cluster

Warden includes `xtask` commands to run a local multi-process cluster.

## Start

```bash
cargo xtask cluster run --nodes 3 --start-port 7443
```

## Status

```bash
cargo xtask cluster status
```

## Stop

```bash
cargo xtask cluster stop
```

## Optional e2e

```bash
cargo xtask e2e --api http://127.0.0.1:7443 --runtime containerd
```
