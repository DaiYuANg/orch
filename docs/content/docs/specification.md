## Workload DSL Specification (MVP)

This is the current minimal DSL contract supported by Warden.

### Top-level fields

```yaml
name: my-app                     # required
description: optional text
include: []                      # optional, legacy alias of includes
includes: []                     # optional
datacenters: []                  # optional
units: []                        # required, at least one
```

### Unit fields

```yaml
units:
  - name: backend                # required
    tasks: []                    # required, at least one
```

### Task fields

```yaml
tasks:
  - name: api                    # required
    type: service                # required
    driver: docker               # required: docker|containerd|systemd|firecracker|windows-service
    stateful: false              # optional, default false
    image: nginx:latest          # required when driver=docker|containerd, and used as rootfs path for firecracker
    command: ["nginx", "-g", "daemon off;"]   # optional
    replicas: 1                  # optional, default 1
    env:                         # optional
      PORT: "8080"
    tags: []                     # optional, key=value style labels
    labels:                      # optional, preferred for routing labels
      key: value
    network:                     # optional
      name: api-net
      port:
        http: 8080
```

### Ingress labels (integrated by default)

Warden ingress is built in. User only writes labels in DSL; no external gateway selection is required.

Supported styles:

1. `warden.*` native labels

```yaml
labels:
  warden.ingress.http.enable: "true"
  warden.ingress.http.host: "api.warden.local"
  warden.ingress.http.path: "/"
  warden.ingress.http.port: "8080"
```

2. Traefik-compatible subset

```yaml
labels:
  traefik.enable: "true"
  traefik.http.routers.api.rule: "Host(`api.warden.local`) && PathPrefix(`/`)"
  traefik.http.services.api.loadbalancer.server.port: "8080"
```

`tags` also supports key-value syntax: `["traefik.enable=true", "..."]`.

### Health check fields

```yaml
check:
  type: http                     # required if check exists, allowed: http|cmd
  path: /health                  # required when type=http
  command: "..."                 # required when type=cmd
  interval: 10s                  # optional, default 10s
  timeout: 3s                    # optional, default 3s
  retries: 3                     # optional, default 3
```

### Runtime behavior (MVP)

- Deployment API parses DSL (`yaml` / `hcl`) and validates required fields.
- Each task replica maps to one runtime instance resolved by `driver`.
- Every running instance is registered into the service directory (endpoint + health + route metadata).
- DNS resolves `<service>.warden.local.` from the service directory.
- Ingress resolves HTTP/TCP/UDP routes from the same service directory and proxies automatically.
- Container restart is automatic when:
  - container exits unexpectedly, or
  - HTTP health check fails continuously until retry threshold.
- Runtime-managed instances are recovered on restart when runtime supports label-based list/recovery.

### Stateful migration guardrail (baseline)

- Mark task as stateful with `stateful: true`.
- Migration/failover/rebalance for stateful deployment requires explicit confirmation:
  - `force_stateful=true`
  - `max_unavailable=1` (current baseline restriction)

### Current API

- `POST /tasks/deploy`
  - Body:
    - `content` (required): DSL text
    - `format` (optional): `yaml` or `hcl`
    - `filename` (optional): used for format detection
- `GET /tasks`
- `GET /tasks/{id}`
- `POST /tasks/{id}/stop`
- `GET /tasks/instances/{id}/logs?tail=200`
