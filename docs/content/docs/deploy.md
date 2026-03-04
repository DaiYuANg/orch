# Deploy

## Minimal Runnable Demo

### 1. Start server

```powershell
go run ./cmd/server run
```

Default HTTP API listens on `http://127.0.0.1:7443`.

### 2. Get root token

Start a new terminal and print the token:

```powershell
go run ./cmd/cli token
```

Or print token file path only:

```powershell
go run ./cmd/cli token --path
```

By default token is generated to:

- Windows: `%TEMP%\warden.token`
- Linux/macOS: `/tmp/warden.token`

### 3. Deploy sample workload

Repo already includes `examples/minimal-nginx.yaml`.

```powershell
go run ./cmd/cli service deploy `
  --file ./examples/minimal-nginx.yaml `
  --api http://127.0.0.1:7443
```

### 4. Verify deployment

List deployments:

```powershell
go run ./cmd/cli service list --api http://127.0.0.1:7443
```

Get one deployment detail:

```powershell
go run ./cmd/cli service get <deployment-id> --api http://127.0.0.1:7443
```

Query system info:

```powershell
go run ./cmd/cli info --api http://127.0.0.1:7443
```

### 5. View logs and stop

```powershell
go run ./cmd/cli service logs <instance-id> --tail 200 --api http://127.0.0.1:7443
go run ./cmd/cli service stop <deployment-id> --api http://127.0.0.1:7443
```

### 6. Placement operations (multi-node)

Migrate a deployment to a target worker node:

```powershell
go run ./cmd/cli service migrate <deployment-id> --target-node <node-id> --api http://127.0.0.1:7443
```

For stateful workloads (`task.stateful: true`), explicit confirmation is required:

```powershell
go run ./cmd/cli service migrate <deployment-id> --target-node <node-id> --force-stateful --max-unavailable 1 --api http://127.0.0.1:7443
```

Fail over all deployments from a failed node:

```powershell
go run ./cmd/cli service failover --failed-node <node-id> --target-node <node-id> --api http://127.0.0.1:7443
go run ./cmd/cli service failover --failed-node <node-id> --force-stateful --max-unavailable 1 --api http://127.0.0.1:7443
```

Trigger a rebalance run:

```powershell
go run ./cmd/cli service rebalance --max-migrations 5 --api http://127.0.0.1:7443
go run ./cmd/cli service rebalance --max-migrations 5 --force-stateful --max-unavailable 1 --api http://127.0.0.1:7443
```
