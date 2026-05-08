# Local Docker DNS Smoke Test

This smoke test verifies workload-to-workload DNS through orch DNS:

```text
orch-server DNS :53 -> Docker DNS injection -> dns-client -> dns-backend.default.svc.orch.local
```

## Prerequisites

- Go toolchain available on `PATH`
- PowerShell (`pwsh`)
- Docker CLI available on `PATH`
- A running Docker engine
- Port `53` on the host available for the smoke server

## Run

```powershell
task smoke:local-docker-dns
```

The script builds local binaries under `.orch-dns-smoke/bin`, starts
`orch-server` on `http://127.0.0.1:17444`, enables orch DNS on `0.0.0.0:53`,
deploys `examples/local-docker-dns-smoke.yaml`, then waits for the client
container to log `orch-dns-ok`. It also prints `get apps`, `describe app`,
`get workloads`, and `get assignments` so the deploy and DNS status can be
checked from the CLI.

Equivalent direct command:

```powershell
pwsh -NoProfile -ExecutionPolicy Bypass -File scripts/local-docker-dns-smoke.ps1
```

The script auto-detects the Docker host IP that containers can use as their DNS
nameserver. Override it when needed:

```powershell
pwsh -NoProfile -ExecutionPolicy Bypass -File scripts/local-docker-dns-smoke.ps1 -WorkloadNameserver 192.168.65.254
```

## What It Deploys

- `dns-backend`: BusyBox `httpd` serving `orch-dns-ok` on port `8080`
- `dns-client`: BusyBox `wget` loop against
  `http://dns-backend.default.svc.orch.local:8080`

By default the script deletes the app and removes both smoke containers before
exiting. Use `-KeepServer -KeepContainer` to inspect the environment manually.
