# orch

orch is a lightweight runtime and control layer for long-lived services (database, message queue, object storage, and other always-on workloads) outside traditional container orchestration systems.

## Status

orch is being built in Go and focuses on:

- Multi-runtime workload execution (`docker`, `containerd`, `firecracker`)
- Built-in ingress and DNS record lifecycle
- Raft-aware scheduling (deploy/migrate/failover/rebalance)
- Declarative Workload DSL direction with compatibility inputs and `plan/render/apply/delete`

Current core stack:

- DI: `github.com/arcgolabs/dix`
- Logging: `github.com/arcgolabs/logx`
- HTTP server/API: `github.com/arcgolabs/httpx` (`fiber` adapter)
- Ingress: Fiber + `middleware/proxy` (round-robin); optional **Let's Encrypt** via `ingress.tls` (`golang.org/x/crypto/acme/autocert`, TLS-ALPN-01).
- CLI: `github.com/spf13/cobra`
- Consensus path: `github.com/hashicorp/raft`

## Project Structure

```text
cmd/
  orch-cli/              # cobra entrypoint
  orch-server/           # process entrypoint + graceful shutdown
internal/
  config/                # env-based config
  logging/               # logx factory
  security/auth/         # authx guard module
  httpserver/            # httpx bootstrap (adapter/fiber + openapi)
  api/                   # typed route registration
  deploy/v1alpha1/       # canonical deploy YAML model
  runtime/
    docker/              # docker provider (v1)
    containerd/          # containerd provider (v1)
  services/
    registry/            # workload registry
    task/                # deploy orchestration service
  raftsvc/               # hashicorp/raft service boundary
```

## Quick Start

### Parse a deploy YAML file

```bash
go run ./cmd/orch-cli parse --file path/to/app.yaml --json
```

### Run server (skeleton)

```bash
go run ./cmd/orch-server
```

### Worker dispatch (dev)

When placement selects a node other than the local server, orch dispatches the workload to that node's worker API. Configure node IDs to API base URLs with `cluster.nodes`:

```yaml
cluster:
  nodes:
    node-a: http://10.0.0.11:17443
    node-b: http://10.0.0.12:17443
  # Optional when HTTP auth is enabled on worker nodes:
  worker_token: "<bearer-token>"
```

The worker endpoint executes the assigned workload locally and does not mutate Raft desired state.
The scheduler records workload assignment results in Raft; inspect them with `orch assignments --json` or `GET /api/v1/assignments`.

## Documentation (mdBook)

Docs are now maintained with `mdBook`.

```bash
mdbook serve docs
```

See:

- [Introduction](docs/introduction.md)
- [Project Status](docs/project-status.md)
- [Quick Start](docs/quick-start.md)
- [Workload DSL v1 (EN)](docs/dsl.md)
- [Workload DSL v1（中文）](docs/dsl.zh.md)
- [Ingress Design v1 (EN)](docs/ingress.md)
- [Ingress 设计 v1（中文）](docs/ingress.zh.md)
- [Local Raft Cluster](docs/local-raft.md)

## Development

```bash
go mod tidy
go test ./...
```

### Release builds (binaries, `.deb`, `.rpm`, `.apk`)

Recommended workflow is **[GoReleaser](https://goreleaser.com)** with **[nFPM](https://nfpm.goreleaser.com)** (configured in `.goreleaser.yaml`): cross-compile **orch** / **orch-server**, tar/zip archives, checksums, and Linux packages (`deb` / `rpm` / `apk`) in one pipeline.

```bash
task goreleaser-check              # validate config
task release-snapshot              # outputs under dist/ (no tag required)
# tagged release + GitHub publish (when wired): goreleaser release --clean
```

Other common approaches:

| Approach | Notes |
|----------|--------|
| **GoReleaser** (this repo) | Single YAML; archives + nfpm packages + optional Homebrew/Chocolatey/GitHub Releases |
| **[nfpm](https://github.com/goreleaser/nfpm) alone** | Only packaging step: you build binaries (`go build`), nfpm produces deb/rpm/apk from `nfpm.yaml` |
| **[fpm](https://github.com/jordansissel/fpm)** | Flexible Ruby-based packager; many formats, extra tooling |
| **rpmbuild / debhelper** | Classic distro-native specs (`*.spec`, `debian/`) — best when targeting specific distributions |
| **OBS / COPR / PPA** | Build RPM/DEB on shared infra for multiple base OS versions |

Full matrix builds must succeed (`go test ./...` and cross-compiles); narrow `builds` targets in `.goreleaser.yaml` if you need to iterate before fixing every GOOS/GOARCH combination.

### Dev Container (VS Code / Cursor / Codespaces)

Open the repo in a container using `.devcontainer/`: Go 1.26 (bookworm), `task`, Delve, and the **Docker CLI** are preinstalled; port `17443` is forwarded for `orch-server` HTTP. After the container builds, `postCreateCommand` runs `go mod download` and `docker version`. For mdBook locally, install a [release binary](https://github.com/rust-lang/mdBook/releases) or use mdBook on the host.

**Docker vs nested runtimes**

- **Default (Docker-from-Docker):** the devcontainer [docker-outside-of-docker](https://github.com/devcontainers/features/tree/main/src/docker-outside-of-docker) feature mounts the **host** Docker socket. Commands like `docker run` / `docker compose` use **your machine’s Docker engine** (e.g. Docker Desktop on Windows/macOS). That is enough to exercise the **docker** runtime provider without running a second daemon inside the devcontainer.
- **Docker-in-Docker:** only if you need an **isolated** daemon (no host socket), swap the feature for `ghcr.io/devcontainers/features/docker-in-docker` and add `"runArgs": ["--privileged"]` (heavier; can be awkward with Docker Desktop). See the [feature docs](https://github.com/devcontainers/features/tree/main/src/docker-in-docker).
- **containerd provider:** orch talks to **containerd’s API/socket**, not the same path as “Docker CLI + host engine”. Nested **containerd + nerdctl** inside a devcontainer is possible but heavy (extra daemons, Linux-only nuances). Typical approaches: run containerd-backed tests on a **Linux host/VM/CI** with `/run/containerd/containerd.sock` (or your distro path) reachable, or develop the docker path in devcontainer and run containerd integration elsewhere.

## Ingress Configuration

Environment variables use the **`ORCH`** prefix (see `internal/config`, loaded via configx).

- `ORCH_INGRESS_ENABLED` (default: `true`)
- Default listeners: `:80` and `:443`. Set `ORCH_INGRESS_ADDR` to use a single port instead (for example `:8088` on machines where binding low ports requires elevated privileges).

## License

MIT
