# Local Docker Worker Dispatch Smoke Test

This smoke test verifies remote worker dispatch without requiring a full
multi-node Raft cluster:

```text
orch-cli -> scheduler orch-server -> worker API -> worker orch-server -> Docker runtime
```

## Prerequisites

- Go toolchain available on `PATH`
- PowerShell (`pwsh`)
- Docker CLI available on `PATH`
- A running Docker engine
- Ports `17445` and `17446` available on the host

## Run

```powershell
task smoke:local-docker-worker-dispatch
```

The script builds local binaries under `.orch-worker-dispatch-smoke/bin`, starts
two `orch-server` processes, configures the scheduler with
`worker-node=http://127.0.0.1:17446`, deploys
`examples/local-docker-worker-dispatch.yaml`, and verifies that the workload is
recorded as running on `worker-node`.

Equivalent direct command:

```powershell
pwsh -NoProfile -ExecutionPolicy Bypass -File scripts/local-docker-worker-dispatch-smoke.ps1
```

## What It Deploys

- Scheduler server: `http://127.0.0.1:17445`, node ID `scheduler-node`
- Worker server: `http://127.0.0.1:17446`, node ID `worker-node`
- App: `dispatch-smoke`
- Workload: `remote-worker`, a long-lived BusyBox container

The scheduler node receives the CLI deploy request, selects the configured
preferred worker node, dispatches the workload through the worker API, and then
serves `get apps`, `describe app`, `get workloads`, and `get assignments` from
the scheduler's view. The script also verifies `stop`, `start`, and `delete`.

By default the script deletes the app, removes the Docker container, and stops
both servers before exiting. Use `-KeepServer -KeepContainer` to inspect the
environment manually.
