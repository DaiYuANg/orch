package api_test

import (
	"testing"

	"github.com/daiyuang/orch/internal/workloadmeta"
)

func TestDeploySourceDispatchesWorkerAndExposesState(t *testing.T) {
	t.Parallel()

	flow := newDeployFlowHarness(t)
	flow.assertRaftReady()

	flow.deploySource()
	flow.expectDeployDispatch("deploy")
	flow.assertRemoteOnly()
	flow.assertRunningState()

	flow.stopDeploy()
	flow.expectStopDispatch("stop")
	flow.assertStoppedState()

	flow.startDeploy()
	flow.expectDeployDispatch("start")
	flow.assertRunningState()

	flow.restartDeploy()
	flow.expectStopDispatch("restart stop")
	flow.expectDeployDispatch("restart start")
	flow.assertRunningState()

	flow.deleteDeploy()
	flow.expectStopDispatch("delete")
	flow.assertDeletedState()
}

func (h *deployFlowHarness) assertRunningState() {
	h.t.Helper()
	assignment := waitHTTPAssignment(h.ctx, h.t, h.client, workloadmeta.AssignmentStatusRunning)
	if assignment.Node != deployFlowRemoteNode || assignment.Status != workloadmeta.AssignmentStatusRunning || assignment.Artifact != deployFlowImage {
		h.t.Fatalf("assignment = %#v", assignment)
	}
	workload := waitHTTPWorkload(h.ctx, h.t, h.client, deployFlowWorkload)
	if workload.Node != deployFlowRemoteNode || workload.Status != workloadmeta.AssignmentStatusRunning || workload.Artifact != deployFlowImage {
		h.t.Fatalf("workload = %#v", workload)
	}
	appStatus := waitHTTPApp(h.ctx, h.t, h.client, workloadmeta.AssignmentStatusRunning)
	if appStatus.Running != 1 || appStatus.DesiredWorkloads != 1 || appStatus.Workloads.Len() != 1 {
		h.t.Fatalf("app status = %#v", appStatus)
	}
}

func (h *deployFlowHarness) assertStoppedState() {
	h.t.Helper()
	assignment := waitHTTPAssignment(h.ctx, h.t, h.client, workloadmeta.AssignmentStatusStopped)
	if assignment.Status != workloadmeta.AssignmentStatusStopped {
		h.t.Fatalf("stopped assignment = %#v", assignment)
	}
	waitHTTPWorkloadGone(h.ctx, h.t, h.client, deployFlowWorkload)
	waitHTTPApp(h.ctx, h.t, h.client, workloadmeta.AssignmentStatusStopped)
}

func (h *deployFlowHarness) assertDeletedState() {
	h.t.Helper()
	assignment := waitHTTPAssignment(h.ctx, h.t, h.client, workloadmeta.AssignmentStatusStopped)
	if assignment.Status != workloadmeta.AssignmentStatusStopped {
		h.t.Fatalf("deleted assignment = %#v", assignment)
	}
	waitHTTPWorkloadGone(h.ctx, h.t, h.client, deployFlowWorkload)
	waitHTTPAppGone(h.ctx, h.t, h.client, deployFlowNamespace, deployFlowApp)
}
