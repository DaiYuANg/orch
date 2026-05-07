# Full-Stack Docker Example

This page shows how a complete application can be expressed with the native
`.orch` DSL and deployed through the current `docker` provider.

The example manifest is:

```text
examples/fullstack-docker.orch
```

It models four workloads:

| Workload | Kind | Runtime | Role |
|----------|------|---------|------|
| `frontend` | `service` | `docker` | Public web UI |
| `backend` | `service` | `docker` | HTTP API |
| `postgres` | `stateful` | `docker` | Database |
| `redis` | `stateful` | `docker` | Cache / queue |

## Validate The Manifest

```bash
go run ./cmd/orch-cli validate --file examples/fullstack-docker.orch
go run ./cmd/orch-cli parse --file examples/fullstack-docker.orch --json
```

The DSL lowers to the canonical `internal/deploy/v1alpha1.App` model. The
server receives the same model whether the source is `.orch` or YAML.

## Deploy Flow

Create the Docker network used by the workloads:

```bash
docker network create orch-demo
```

Start the server. If you want ingress without binding privileged ports, use a
high port:

```bash
go run ./cmd/orch-server --ingress-listen :18080
```

Deploy and wait:

```bash
go run ./cmd/orch-cli apply --file examples/fullstack-docker.orch --watch
go run ./cmd/orch-cli get workloads
go run ./cmd/orch-cli get assignments
```

Replace these placeholder images before running a real deploy:

```text
ghcr.io/acme/fullstack-backend:latest
ghcr.io/acme/fullstack-frontend:latest
```

## DSL Shape

The backend workload demonstrates the core wiring:

```plano
workload backend {
  kind = "service"
  runtime = "docker"
  depends_on = [postgres, redis]

  run {
    image = "ghcr.io/acme/fullstack-backend:latest"
  }

  env {
    name = "DATABASE_URL"
    value = "postgres://orch:orch-dev-password@orch-demo-postgres:5432/app?sslmode=disable"
  }

  runtime_options {
    docker {
      network_mode = "orch-demo"
    }
  }

  endpoint backend_http {
    port = 8080
    protocol = "http"
  }
}
```

The ingress block routes public traffic to HTTP endpoints declared by workloads:

```plano
ingress public {
  route {
    path = "/api"
    backend_workload = "backend"
    backend_endpoint = "backend_http"
  }

  route {
    path = "/"
    backend_workload = "frontend"
    backend_endpoint = "frontend_http"
  }
}
```

## Current Docker Provider Boundaries

The manifest intentionally stays inside the current runtime surface:

- Supported by the `docker` provider now: image pull, command, args, env, cwd,
  resource limits, `runtime_options.docker.network_mode`, and `privileged`.
- `depends_on` is a scheduling/deploy graph signal today; it is not yet a
  readiness gate.
- Workload `endpoint` entries feed DNS/ingress intent; they do not publish host
  ports directly.
- The deploy model has volumes/mounts, but the current `docker` provider does
  not mount them yet. Treat `postgres` and `redis` in this example as topology
  examples until volume mounting is wired.
- Endpoint symbols are currently global in the `.orch` compiler, so use unique
  endpoint labels such as `backend_http` and `frontend_http`.

For a smoke test that runs without placeholder app images, use
`examples/local-docker-smoke.yaml` and `task smoke:local-docker`.
