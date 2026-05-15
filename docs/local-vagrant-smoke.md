# Local Vagrant Smoke Test

This script verifies a reproducible 3-node control-plane flow:

```text
vagrant up -> install orch binaries -> deploy -> wait ready -> verify -> delete app
```

It exercises the full deploy path on a real three-node virtualized cluster
using Docker runtime on Linux VMs.

## Prerequisites

- Go toolchain on host PATH
- PowerShell (`pwsh`)
- Vagrant
- On Windows: Hyper-V (requires Administrator when running Vagrant)
- On macOS/Linux: VirtualBox
- Linux VM images supporting Docker package install (Ubuntu/Debian by default)

## Run

```powershell
task smoke:vagrant
```

Provider selection is automatic:

- Windows hosts default to `hyperv`
- Non-Windows hosts default to `virtualbox`

You can override with:

```powershell
$env:ORCH_VAGRANT_PROVIDER="virtualbox"
task smoke:vagrant
```

or

```powershell
$env:VAGRANT_DEFAULT_PROVIDER="hyperv"
task smoke:vagrant
```

Forward script flags through Task with `--`, for example:

```powershell
task smoke:vagrant -- -KeepNodes -WorkDir ".orch-vagrant" -TimeoutSeconds 240
```

Equivalent direct command:

```powershell
pwsh -NoProfile -ExecutionPolicy Bypass -File scripts/vagrant/smoke.ps1
```

## Script Parameters

- `-Manifest`: manifest path (default `examples/local-vagrant-smoke.yaml`)
- `-WorkDir`: working directory synced from host to `/vagrant` (default `.orch-vagrant`)
- `-AppName`: app name to deploy (default `vagrant-smoke`)
- `-WorkloadName`: workload name to wait for (default `smoke-worker`)
- `-TimeoutSeconds`: timeout for each wait step (default `180`)
- `-KeepNodes`: keep VMs running after test completes
- `-DestroyNodes`: destroy VMs after test completes
- `-SkipBuild`: skip host-side `go build` and reuse existing artifacts in `WorkDir`

By default, nodes are halted on completion to make reruns faster while keeping
artifacts and VM state intact.

## What It Does

On completion, the script creates and starts a three-node Vagrant cluster:

- `node1`: `192.168.56.11` (`:17443`)
- `node2`: `192.168.56.12` (`:17444`)
- `node3`: `192.168.56.13` (`:17445`)

Each node gets an installed `orch-server` and service setup via
`scripts/vagrant/orch-node-setup.sh`; ingress and observability exporters are
disabled by default for deterministic smoke behavior.

It then:

- Builds Linux/host binaries under `WorkDir/dist/bin`
- Deploys `examples/local-vagrant-smoke.yaml`
- Waits for the workload to become running and visible on cluster assignment
- Prints app/workload/assignment views
- Deletes the app and waits for it to disappear from cluster state

## Node image customization

Provider-specific defaults are applied:

- Windows/Hyper-V: `generic/ubuntu2204`
- Other providers (VirtualBox): `ubuntu/jammy64`

You can override these with env vars:

```powershell
$env:ORCH_VAGRANT_BOX = "archlinux/archlinux"
task smoke:vagrant
```

Or provider-specific override:

```powershell
$env:ORCH_VAGRANT_PROVIDER="hyperv"
$env:ORCH_VAGRANT_BOX_HYPERV="generic/ubuntu2404"
task smoke:vagrant
```

Override Docker install channel/arch for Debian-family nodes through
`ORCH_VAGRANT_DOCKER_CHANNEL` and `ORCH_VAGRANT_DOCKER_ARCH`.
