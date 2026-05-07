# Full-Stack Docker Example

This page shows how a complete application can be expressed with the short
native `.orch` DSL and deployed through the current `docker` provider.

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

The example uses the short authoring form. App-level Docker defaults are
inherited by shorthand workloads, `service` and `stateful` imply workload kind,
`env = { ... }` replaces repeated env blocks, and `http(...)` / `tcp(...)`
declare endpoints:

```plano
app {
  name = "fullstack"
  namespace = "demo"

  docker {
    network = "orch-demo"
  }

  stateful postgres {
    image = "postgres:16-alpine"
    env = {
      POSTGRES_DB = "app",
      POSTGRES_USER = "orch",
      POSTGRES_PASSWORD = "orch-dev-password",
    }

    tcp(5432)
    resources = "500m/512Mi"
  }

  service backend {
    image = "ghcr.io/acme/fullstack-backend:latest"
    depends_on = [postgres, redis]
    env = {
      DATABASE_URL = "postgres://orch:orch-dev-password@orch-demo-postgres:5432/app?sslmode=disable",
      REDIS_ADDR = "orch-demo-redis:6379",
      HTTP_ADDR = ":8080",
    }

    http(8080)
    resources = "500m/512Mi"
  }
}
```

The full manifest also defines `redis`, `frontend`, and ingress. The lowerer
turns the short form into the same canonical deploy model as the verbose form:

```text
service backend {
  image = "..."
  depends_on = [postgres, redis]
  env = { HTTP_ADDR = ":8080" }
  http(8080)
  resources = "500m/512Mi"
}
```

Ingress can use the compact `path` form. If the workload has exactly one HTTP
endpoint, the endpoint name is inferred:

```plano
ingress public {
  path "/api" {
    workload = backend
  }

  path "/" {
    workload = frontend
  }
}
```

Use `endpoint = "name"` inside `path` when the workload has multiple HTTP
endpoints or when routing to a non-default endpoint name.

## Current Docker Provider Boundaries

The manifest intentionally stays inside the current runtime surface:

- Supported by the `docker` provider now: image pull, command, args, env, cwd,
  resource limits, Docker network mode, labels, and `privileged`.
- `depends_on` is a scheduling/deploy graph signal today; it is not yet a
  readiness gate.
- Workload `endpoint` entries feed DNS/ingress intent; they do not publish host
  ports directly.
- The deploy model has volumes/mounts, but the current `docker` provider does
  not mount them yet. Treat `postgres` and `redis` in this example as topology
  examples until volume mounting is wired.
- The verbose `workload { run { ... } endpoint { ... } }` form still works as
  an escape hatch when the short form is not expressive enough.

For a smoke test that runs without placeholder app images, use
`examples/local-docker-smoke.yaml` and `task smoke:local-docker`.
