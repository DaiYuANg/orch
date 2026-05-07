# Local Docker Smoke Test

This smoke test exercises the full local deploy path:

```text
orch-server -> orch-cli apply -> Docker runtime -> orch-cli workloads / assignments
```

## Prerequisites

- Go toolchain available on `PATH`
- PowerShell (`pwsh`)
- Docker CLI available on `PATH`
- A running Docker engine

## Run

```powershell
task smoke:local-docker
```

Forward script flags through Task with `--`, for example:

```powershell
task smoke:local-docker -- -KeepServer -KeepContainer
```

Equivalent direct command:

```powershell
pwsh -NoProfile -ExecutionPolicy Bypass -File scripts/local-docker-smoke.ps1
```

The script builds local binaries under `.orch-smoke/bin`, starts `orch-server` on
`http://127.0.0.1:17443`, deploys `examples/local-docker-smoke.yaml`, then waits
until both views report the workload as running:

```powershell
.orch-smoke/bin/orch --server http://127.0.0.1:17443 workloads
.orch-smoke/bin/orch --server http://127.0.0.1:17443 assignments
```

By default the script removes the smoke Docker container and stops the server
before exiting.

## Keep The Environment Running

Use this when you want to inspect the server manually after the script verifies
the deploy:

```powershell
pwsh -NoProfile -ExecutionPolicy Bypass -File scripts/local-docker-smoke.ps1 -KeepServer -KeepContainer
```

Then run:

```powershell
.orch-smoke/bin/orch --server http://127.0.0.1:17443 workloads
.orch-smoke/bin/orch --server http://127.0.0.1:17443 assignments
docker ps --filter name=orch-default-smoke
```

Cleanup:

```powershell
docker rm -f orch-default-smoke
```

Stop the retained `orch-server` process from Task Manager or with PowerShell
after finding it:

```powershell
Get-Process orch-server
Stop-Process -Name orch-server
```

## What It Starts

The smoke server is intentionally single-node and low-risk:

- Raft disabled
- Ingress disabled
- DNS disabled
- Prometheus and OTLP disabled
- Node ID forced to `smoke-node`

The example workload is a long-lived BusyBox container named
`orch-default-smoke`.
