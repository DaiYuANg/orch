# Beta Release

This page defines the release bar for `v0.1.0-beta.*`.

## Supported Beta Scope

The beta is intended for developer trials and small controlled environments.

Supported paths:

- CLI/server binaries for Linux, macOS, and Windows.
- Linux packages through GoReleaser/nFPM: `.deb`, `.rpm`, and `.apk`.
- Docker runtime deploy lifecycle: `apply`, `get`, `describe`, `stop`, `start`,
  `restart`, and `delete`.
- Worker dispatch between scheduler and worker server processes.
- Workload DNS for supported container paths, with configurable upstream DNS.
- Built-in HTTP ingress through `github.com/arcgolabs/vale`.
- Static Raft bootstrap, basic add/remove voter operations, and follower write
  forwarding when `cluster.nodes` maps leader IDs to API URLs.
- Baseline explicit `migrate`, `failover`, and `rebalance` operations.

Experimental in beta:

- `containerd` CRI runtime status/recovery behavior.
- Firecracker TAP/bridge and image preparation workflow.
- Linux `systemd` runtime and Windows `windows-service` runtime.
- Host DNS installer behavior outside common Linux `systemd-resolved`, macOS
  resolver, and Windows registry/resolver setups.

Not promised in beta:

- Automatic node failure detection and automatic failover.
- Stateful volume/data migration.
- Raft quorum safety guardrails for every membership edit.
- Production hardening for rolling upgrades and rollback.
- Full TCP/UDP ingress parity.

## Release Gate

Run these before tagging:

```bash
go mod tidy
go test ./...
task goreleaser-check
task release-snapshot
```

Run these smoke tests on a host with the required runtime:

```powershell
task smoke:local-raft-forwarding
task smoke:local-docker
task smoke:local-docker-worker-dispatch
task smoke:local-docker-dns
```

`smoke:local-docker-dns` requires Docker and the host DNS port used by the smoke
server to be available.

## Tagging

Use prerelease semver tags:

```bash
git tag -a v0.1.0-beta.1 -m "v0.1.0-beta.1"
git push origin v0.1.0-beta.1
```

Pushing a `v*` tag runs `.github/workflows/release.yml`, which publishes
archives, checksums, and Linux packages through GoReleaser.

## Manual Snapshot

For a local dry run without publishing:

```bash
task release-snapshot
```

Artifacts are written to `dist/`.
