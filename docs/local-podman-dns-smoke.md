# Local Podman DNS Smoke Test

This smoke test verifies workload-to-workload DNS through orch DNS using Podman:

```text
orch-server DNS :53 -> Podman workload DNS injection -> dns-client -> dns-backend.default.svc.orch.local
```

## Prerequisites

- Go toolchain available on `PATH`
- PowerShell (`pwsh`)
- Podman CLI available on `PATH`
- Port `53` on the host available for the smoke server

## Run

```powershell
task smoke:local-podman-dns
```

Equivalent direct command:

```powershell
pwsh -NoProfile -ExecutionPolicy Bypass -File scripts/local-docker-dns-smoke.ps1 -ContainerRuntime podman -Manifest examples/local-podman-dns-smoke.yaml
```

If auto-detection fails, pass a nameserver explicitly:

```powershell
pwsh -NoProfile -ExecutionPolicy Bypass -File scripts/local-docker-dns-smoke.ps1 -ContainerRuntime podman -Manifest examples/local-podman-dns-smoke.yaml -WorkloadNameserver 192.168.65.254
```

## Notes

The script starts `orch-server` on `http://127.0.0.1:17444`, enables orch DNS on
`0.0.0.0:53`, deploys `examples/local-podman-dns-smoke.yaml`, and waits until
the client reports `orch-dns-ok`.
