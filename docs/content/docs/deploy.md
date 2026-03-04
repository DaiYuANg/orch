# Deploy

## Minimal Runnable Demo

### 1. Start server

```powershell
go run ./cmd/server server
```

Default HTTP API listens on `http://127.0.0.1:7443`.

### 2. Get root token

Start a new terminal and print the token:

```powershell
go run ./cmd/server token
```

Or print token file path only:

```powershell
go run ./cmd/server token --path
```

By default token is generated to:

- Windows: `%TEMP%\warden.token`
- Linux/macOS: `/tmp/warden.token`

### 3. Deploy sample workload

Repo already includes `examples/minimal-nginx.yaml`.

```powershell
go run ./cmd/server service deploy `
  --file ./examples/minimal-nginx.yaml `
  --api http://127.0.0.1:7443
```

### 4. Verify deployment

List deployments:

```powershell
go run ./cmd/server service list --api http://127.0.0.1:7443
```

Get one deployment detail:

```powershell
go run ./cmd/server service get <deployment-id> --api http://127.0.0.1:7443
```

Query system info:

```powershell
go run ./cmd/server info --api http://127.0.0.1:7443
```

### 5. View logs and stop

```powershell
go run ./cmd/server service logs <instance-id> --tail 200 --api http://127.0.0.1:7443
go run ./cmd/server service stop <deployment-id> --api http://127.0.0.1:7443
```
