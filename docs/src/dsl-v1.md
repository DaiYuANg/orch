# DSL v1

Warden DSL v1 provides declarative workload management in YAML.

Manifest type:

```yaml
apiVersion: warden.io/v1alpha1
kind: Application
metadata:
  name: demo
  namespace: default
spec:
  workloads:
    - name: web
      runtime: containerd
      image: docker.io/library/nginx:stable-alpine
```

Reference manifest: `examples/dsl-v1-demo.yaml`.

## Commands

Plan (summary):

```bash
cargo run -p warden-cli-rs -- --api http://127.0.0.1:7443 dsl plan --file examples/dsl-v1-demo.yaml
```

Plan (JSON):

```bash
cargo run -p warden-cli-rs -- --api http://127.0.0.1:7443 dsl plan --file examples/dsl-v1-demo.yaml --json
```

Plan strict:

```bash
cargo run -p warden-cli-rs -- --api http://127.0.0.1:7443 dsl plan --file examples/dsl-v1-demo.yaml --strict
```

Render compiled deploy requests:

```bash
cargo run -p warden-cli-rs -- dsl render --file examples/dsl-v1-demo.yaml
```

Apply:

```bash
cargo run -p warden-cli-rs -- --api http://127.0.0.1:7443 dsl apply --file examples/dsl-v1-demo.yaml --concurrency 8
```

Apply with prune:

```bash
cargo run -p warden-cli-rs -- --api http://127.0.0.1:7443 dsl apply --file examples/dsl-v1-demo.yaml --prune
```

Delete managed workloads:

```bash
cargo run -p warden-cli-rs -- --api http://127.0.0.1:7443 dsl delete --file examples/dsl-v1-demo.yaml
```

## Notes

- Workload names are compiled to: `<namespace>.<application>.<workload>`.
- Server-side apply endpoint is `POST /dsl/apply`.
- Apply supports bounded concurrency and rollback on create failure.
