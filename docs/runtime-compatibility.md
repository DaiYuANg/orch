# Runtime Compatibility Matrix

Snapshot date: May 18, 2026

This matrix tracks the current provider surface, not the long-term target. "Yes"
means the code path exists in the provider today; it does not imply the runtime
has a dedicated end-to-end smoke test on every operating system.

| Runtime | Host support | Deploy / stop | Status | Logs | orch DNS behavior | Notes |
| --- | --- | --- | --- | --- | --- | --- |
| `docker` | Docker Engine reachable from the server host | Yes | Yes | Yes | Injects orch DNS resolver/search domains into Docker host config and records workload A records | Primary smoke-tested runtime. Works well for local beta flows and worker dispatch. |
| `podman` | Docker-compatible Podman API reachable from the server host | Yes | Yes | Yes | Shares Docker behavior via shared adapter: injects orch DNS resolver/search domains into container config and records workload A records | Runtime kind `podman` currently uses the Docker-compatible runtime adapter and Podman env/host wiring. |
| `containerd` | Linux with containerd CRI plugin and working CNI | Yes | Yes | Yes | Injects orch DNS through CRI sandbox DNS config and records workload A records from sandbox IP | Uses CRI pod sandboxes only. `CONTAINERD_ADDRESS` can override the default socket. `runtimeOptions.containerd.namespace` maps to CRI sandbox metadata only; orch DNS and labels still use `metadata.namespace`. |
| `process` | Host process execution through `os/exec` | Yes | Yes | Yes | Records workload A records to the host-facing workload address; process resolver behavior depends on host DNS installer | Supports custom stdout/stderr log paths under `runtimeOptions.process`. |
| `systemd` | Linux systemd | Yes | Yes | Yes | Records workload A records to the host-facing workload address; service resolver behavior depends on host DNS installer | Status uses systemd DBus. Logs use `journalctl --unit`. |
| `firecracker` | Linux with KVM, Firecracker binary, kernel/rootfs, and prepared networking | Yes | Yes | Yes | Not yet wired to workload DNS records or guest resolver injection | Uses `firecracker-go-sdk`; TAP/bridge automation, jailer, guest DNS, and image preparation remain future hardening. |
| `windows-service` | Windows Service Control Manager | Yes | Fallback only | No | Records workload A records to the host-facing workload address; service resolver behavior depends on host DNS installer | Deploy/stop are implemented on Windows. Runtime-local status/logs still need native SCM/Event Log integration. |

## Provider Discovery And Policy

orch-server probes the local host at startup and registers runtime providers only
when their host dependency is detected. The `process` provider is available by
default. Docker, Podman, containerd, Firecracker, systemd, and Windows Service
are registered when the relevant CLI, socket, OS, or binary is present.

Provider registration can be controlled with `runtime.providers.<name>.policy`:

```yaml
runtime:
  providers:
    docker:
      policy: required # auto | required | disabled
    firecracker:
      policy: disabled
```

- `auto` registers the provider when detected.
- `required` fails server startup when the provider is unavailable.
- `disabled` prevents registration even when the host dependency is detected.

Inspect the local node runtime state with:

```bash
orch get runtimes
orch runtimes --json
orch diagnostics
```

## Control-Plane Features

| Feature | Current behavior |
| --- | --- |
| Local inspect API | `GET /api/v1/workloads/{namespace}/{app}/{workload}/status` and `/logs` read provider status/logs when the workload is local. |
| Remote inspect API | If Raft assignment points to a remote node, the task service forwards status/logs to the worker API configured in `cluster.nodes`. |
| Worker inspect API | Worker nodes expose internal `POST /api/v1/worker/status` and `POST /api/v1/worker/logs` endpoints. |
| CLI | `orch get runtimes` shows provider policy/detection/registration; `orch diagnostics` includes the provider summary; `orch describe workload` uses runtime status; `orch logs` reads provider logs through the control plane. |
| Raft | Single-node durable Dragonboat is the default baseline; static multi-node bootstrap and leader failover are covered by tests. |

## Provider Gaps

- `windows-service` needs native runtime status and log support.
- `firecracker` needs automated network setup, DNS/guest resolver integration,
  recovery, jailer support, and a documented image/rootfs preparation workflow.
- `containerd` has CRI status/logs, but recovery and broader Linux integration
  smoke coverage are still beta hardening work.
- `process` and `systemd` rely on the host DNS installer for resolver behavior;
  they do not inject per-process resolver config.
