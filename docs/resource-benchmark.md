# Resource Benchmark

This benchmark measures orch control-plane process cost on the local machine.
It keeps workload cost separate from server cost so the numbers are useful when
comparing with Kubernetes control-plane components.

Run the default process-runtime scenario:

```powershell
task bench:resources
```

Run with a Docker workload instead of the local process provider:

```powershell
task bench:resources -- -ScheduleRuntime docker
```

The script reports:

- `orch.single.idle`: one `orch-server` with Raft, DNS, HTTP, scheduler, and
  Prometheus endpoint enabled; ingress is disabled to avoid privileged ports.
- `orch.single.schedule_apply.<runtime>`: server resource use while `orch-cli
  apply --watch` deploys one workload.
- `orch.single.scheduled_idle.<runtime>`: the same server after the workload is
  running.
- `orch.cluster3.idle.combined`: three `orch-server` processes with a
  three-voter Dragonboat Raft cluster.
- `orch.cluster3.idle.<node>`: short per-node samples for the three-node
  cluster.

Results are written as JSON under `.orch-resource-bench/results/`.

## Kubernetes Comparison Anchors

Use official Kubernetes distribution requirements as comparison anchors, not as
exact process RSS numbers:

- kubeadm upstream docs require at least 2 GiB RAM per machine and at least 2
  CPUs for a control-plane machine:
  <https://kubernetes.io/docs/setup/production-environment/tools/kubeadm/create-cluster-kubeadm/>
- K3s hardware requirements list 2 cores / 2 GB RAM for a server and 1 core /
  512 MB RAM for an agent:
  <https://docs.k3s.io/installation/requirements>
- K3s resource profiling reports an Intel baseline around 6% of a core and
  roughly 1.6 GB RAM for a server with workload, around 5% of a core and
  roughly 1.4 GB RAM for a server plus one agent, and around 275 MB RAM for an
  agent:
  <https://docs.k3s.io/reference/resource-profiling>
- MicroK8s documents that it can run in about 540 MB of memory, while
  recommending 4 GB RAM for room to run workloads:
  <https://microk8s.io/docs/getting-started>

For a fair Kubernetes comparison, measure on the same host and separate:

- Kubernetes control plane: `kube-apiserver`, `kube-scheduler`,
  `kube-controller-manager`, `etcd`, and distribution-specific agents.
- Node plane: `kubelet`, `kube-proxy` or CNI agents, container runtime, and
  CoreDNS.
- Workload containers themselves.

Compare `orch.cluster3.idle.combined` to the Kubernetes control-plane baseline,
and compare `orch.single.schedule_apply.*` to the scheduler/apply path for an
equivalent one-workload deployment.
