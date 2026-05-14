package api_test

import (
	"time"

	"github.com/lyonbrown4d/orch/internal/workerapi"
	"github.com/lyonbrown4d/orch/internal/workloadmeta"
)

const deployFlowManifest = `metadata:
  name: e2e-demo
  namespace: default
workloads:
  - name: worker
    kind: worker
    runtime: docker
    run:
      artifact:
        image: busybox
    scheduling:
      preferredNodes:
        - node-b
`

func (h *deployFlowHarness) assertRaftReady() {
	h.t.Helper()
	status, err := h.client.RaftStatus(h.ctx)
	requireNoError(h.t, err, "read raft status")
	if !status.Body.Ready || !status.Body.IsLeader || status.Body.State != "Leader" {
		h.t.Fatalf("raft status = %#v", status.Body)
	}
}

func (h *deployFlowHarness) deploySource() {
	h.t.Helper()
	out, err := h.client.DeploySource(h.ctx, "app.yaml", deployFlowManifest)
	requireNoError(h.t, err, "deploy source")
	if !out.Body.Accepted || out.Body.App != deployFlowApp || out.Body.Workloads != 1 {
		h.t.Fatalf("deploy response = %#v", out.Body)
	}
}

func (h *deployFlowHarness) stopDeploy() {
	h.t.Helper()
	out, err := h.client.StopDeploy(h.ctx, deployFlowNamespace, deployFlowApp)
	requireNoError(h.t, err, "stop deploy")
	if !out.Body.Accepted || out.Body.App != deployFlowApp || out.Body.Status != workloadmeta.AssignmentStatusStopped {
		h.t.Fatalf("stop response = %#v", out.Body)
	}
}

func (h *deployFlowHarness) startDeploy() {
	h.t.Helper()
	out, err := h.client.StartDeploy(h.ctx, deployFlowNamespace, deployFlowApp)
	requireNoError(h.t, err, "start deploy")
	if !out.Body.Accepted || out.Body.App != deployFlowApp || out.Body.Status != workloadmeta.AssignmentStatusRunning {
		h.t.Fatalf("start response = %#v", out.Body)
	}
}

func (h *deployFlowHarness) restartDeploy() {
	h.t.Helper()
	out, err := h.client.RestartDeploy(h.ctx, deployFlowNamespace, deployFlowApp)
	requireNoError(h.t, err, "restart deploy")
	if !out.Body.Accepted || out.Body.App != deployFlowApp || out.Body.Status != workloadmeta.AssignmentStatusRunning {
		h.t.Fatalf("restart response = %#v", out.Body)
	}
}

func (h *deployFlowHarness) deleteDeploy() {
	h.t.Helper()
	out, err := h.client.DeleteDeploy(h.ctx, deployFlowNamespace, deployFlowApp)
	requireNoError(h.t, err, "delete deploy")
	if !out.Body.Accepted || out.Body.App != deployFlowApp || out.Body.Status != workloadmeta.AssignmentStatusStopped {
		h.t.Fatalf("delete response = %#v", out.Body)
	}
}

func (h *deployFlowHarness) expectDeployDispatch(action string) {
	h.t.Helper()
	got := receiveDeployWorkload(h, action)
	if got.Node != deployFlowRemoteNode || got.Workload.Name != deployFlowWorkload || got.Workload.Run.Artifact.Image != deployFlowImage {
		h.t.Fatalf("worker %s request = %#v", action, got)
	}
}

func (h *deployFlowHarness) expectStopDispatch(action string) {
	h.t.Helper()
	got := receiveStopWorkload(h, action)
	if got.Node != deployFlowRemoteNode || got.Workload.Name != deployFlowWorkload || got.Metadata.Name != deployFlowApp {
		h.t.Fatalf("worker %s request = %#v", action, got)
	}
}

func (h *deployFlowHarness) assertRemoteOnly() {
	h.t.Helper()
	if h.localRuntime.deployedCount() != 0 {
		h.t.Fatalf("local runtime should not deploy remote workload, got %d deploys", h.localRuntime.deployedCount())
	}
}

func receiveDeployWorkload(h *deployFlowHarness, action string) workerapi.DeployWorkloadBody {
	select {
	case got := <-h.workerCh:
		return got
	case <-time.After(5 * time.Second):
		h.t.Fatalf("timed out waiting for worker %s", action)
		return workerapi.DeployWorkloadBody{}
	}
}

func receiveStopWorkload(h *deployFlowHarness, action string) workerapi.StopWorkloadBody {
	select {
	case got := <-h.stopCh:
		return got
	case <-time.After(5 * time.Second):
		h.t.Fatalf("timed out waiting for worker %s", action)
		return workerapi.StopWorkloadBody{}
	}
}
