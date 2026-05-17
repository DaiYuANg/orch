# Local Podman Worker Dispatch Smoke Test

This smoke test verifies remote worker dispatch with the Podman runtime:

```text
orch-cli -> scheduler orch-server -> worker API -> worker orch-server -> Podman workload
```

## Prerequisites

- Go toolchain available on `PATH`
- PowerShell (`pwsh`)
- Podman CLI available on `PATH`
- Ports `17445` and `17446` available on the host

## Run

```powershell
task smoke:local-podman-worker-dispatch
```

Equivalent direct command:

```powershell
pwsh -NoProfile -ExecutionPolicy Bypass -File scripts/local-docker-worker-dispatch-smoke.ps1 -ContainerRuntime podman -Manifest examples/local-podman-worker-dispatch.yaml
```

The script starts two `orch-server` processes (scheduler + worker), deploys
`examples/local-podman-worker-dispatch.yaml`, and verifies the workload is running
on `worker-node`.
