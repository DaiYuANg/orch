# Local Docker Smoke Test

This smoke test exercises the full local deploy path:

```text
orch-server -> orch-cli apply --watch -> Docker runtime -> orch-cli get workloads / get assignments -> orch-cli start app -> orch-cli stop app -> orch-cli start app -> orch-cli restart app -> orch-cli delete app
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
`http://127.0.0.1:17443`, deploys `examples/local-docker-smoke.yaml`, waits
until both views report the workload as running, stops the app, starts it again
from retained desired state, restarts it, then deletes the app. The first
`start` intentionally runs while the app is already running to verify start is
idempotent. Stop and delete wait until the workload disappears and the assignment
is marked `stopped`:

```powershell
.orch-smoke/bin/orch --server http://127.0.0.1:17443 get workloads
.orch-smoke/bin/orch --server http://127.0.0.1:17443 get assignments
.orch-smoke/bin/orch --server http://127.0.0.1:17443 start app smoke -n default
.orch-smoke/bin/orch --server http://127.0.0.1:17443 stop app smoke -n default
.orch-smoke/bin/orch --server http://127.0.0.1:17443 start app smoke -n default
.orch-smoke/bin/orch --server http://127.0.0.1:17443 restart app smoke -n default
.orch-smoke/bin/orch --server http://127.0.0.1:17443 delete app smoke -n default
```

By default the script verifies deploy, stop, start, restart, and delete, removes
any remaining smoke Docker container, and stops the server before exiting.

## Keep The Environment Running

Use this when you want to inspect the server manually after the script verifies
the deploy. `-KeepContainer` also skips the delete phase so the workload remains
available for manual checks:

```powershell
pwsh -NoProfile -ExecutionPolicy Bypass -File scripts/local-docker-smoke.ps1 -KeepServer -KeepContainer
```

Then run:

```powershell
.orch-smoke/bin/orch --server http://127.0.0.1:17443 get workloads
.orch-smoke/bin/orch --server http://127.0.0.1:17443 get assignments
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
