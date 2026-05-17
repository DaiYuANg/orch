# Local Podman Smoke Test

This smoke test exercises the full local deploy path using the Podman runtime:

```text
orch-server -> orch-cli ready --wait -> orch-cli apply --watch -> orch-cli wait app -> Podman runtime -> orch-cli describe workload / logs / events -> orch-cli start app -> orch-cli stop app -> orch-cli start app -> orch-cli restart app -> orch-cli delete app
```

## Prerequisites

- Go toolchain available on `PATH`
- PowerShell (`pwsh`)
- Podman CLI available on `PATH`
- A running Podman engine/socket

## Run

```powershell
task smoke:local-podman
```

Equivalent direct command:

```powershell
pwsh -NoProfile -ExecutionPolicy Bypass -File scripts/local-docker-smoke.ps1 -ContainerRuntime podman -Manifest examples/local-podman-smoke.yaml
```

The script builds local binaries under `.orch-smoke/bin`, starts `orch-server` on
`http://127.0.0.1:17443`, deploys `examples/local-podman-smoke.yaml`, waits until
the app is running, exercises `describe workload`, `logs`, and `events`, stops the
app, starts it again, restarts it, then deletes it.

## Keep The Environment Running

Use this when you want to inspect the server manually after the script verifies the
deploy. `-KeepContainer` also skips the delete phase so the workload remains for
manual checks:

```powershell
pwsh -NoProfile -ExecutionPolicy Bypass -File scripts/local-docker-smoke.ps1 -ContainerRuntime podman -Manifest examples/local-podman-smoke.yaml -KeepServer -KeepContainer
```

## What It Tests

- Deploy
- Readiness/state polling
- `start`, `stop`, `restart`, and `delete` lifecycle operations
