package task_test

import (
	"context"
	"testing"
	"time"

	"github.com/lyonbrown4d/orch/internal/config"
	"github.com/lyonbrown4d/orch/internal/services/task"
	"github.com/lyonbrown4d/orch/internal/workerapi"
	"github.com/lyonbrown4d/orch/internal/workloadmeta"
)

func TestSubmitMigrateMovesWorkloadToTargetNode(t *testing.T) {
	t.Parallel()

	dispatchCh := make(chan workerapi.DeployWorkloadBody, 1)
	worker := newDeployWorkerServer(t, dispatchCh, workloadmeta.AssignmentStatusRunning)
	cfg := config.Default()
	cfg.Cluster.Nodes = map[string]string{"node-b": worker.URL}
	harness := newTaskHarness(t, cfg, task.NewHTTPWorkerDispatcher(cfg))
	app := deployApp("migrate-demo", dockerWorkload("worker", "busybox"))
	harness.applyApp(t, app)
	harness.applyWorkerAssignment(t, app, "node-a", workloadmeta.AssignmentStatusRunning)

	summary, err := harness.svc.SubmitMigrate(context.Background(), app.Metadata, task.AppOperationOptions{TargetNode: "node-b"})
	if err != nil {
		t.Fatal(err)
	}
	requireMoveSummary(t, summary, 1, "node-b")
	requireWorkerDispatch(t, waitWorkerDispatch(t, dispatchCh, 3*time.Second), "node-b", "worker")
	harness.requireAssignment(t, app, "worker", "node-b", workloadmeta.AssignmentStatusRunning)
}

func TestSubmitFailoverMovesFailedWorkloadToLocalNode(t *testing.T) {
	t.Parallel()

	harness := newTaskHarness(t, config.Default(), nil)
	app := deployApp("failover-demo", dockerWorkload("worker", "busybox"))
	harness.applyApp(t, app)
	harness.applyWorkerAssignment(t, app, "node-b", workloadmeta.AssignmentStatusFailed)

	summary, err := harness.svc.SubmitFailover(context.Background(), app.Metadata, task.AppOperationOptions{})
	if err != nil {
		t.Fatal(err)
	}
	requireMoveSummary(t, summary, 1, "")
	requireWorkloadName(t, harness.waitRuntimeDeploy(t, 3*time.Second), "worker")
	harness.requireAssignment(t, app, "worker", "node-a", workloadmeta.AssignmentStatusRunning)
}

func TestSubmitRebalanceStartsUnassignedWorkloadWithoutStop(t *testing.T) {
	t.Parallel()

	harness := newTaskHarness(t, config.Default(), nil)
	app := deployApp("rebalance-demo", dockerWorkload("worker", "busybox"))
	harness.applyApp(t, app)

	summary, err := harness.svc.SubmitRebalance(context.Background(), app.Metadata, task.AppOperationOptions{})
	if err != nil {
		t.Fatal(err)
	}
	requireRunningMoveSummary(t, summary, 1)
	harness.requireNoStop(t)
	requireWorkloadName(t, harness.waitRuntimeDeploy(t, 3*time.Second), "worker")
	harness.requireAssignment(t, app, "worker", "node-a", workloadmeta.AssignmentStatusRunning)
}

func requireMoveSummary(t *testing.T, summary task.AppOperationSummary, moved int, targetNode string) {
	t.Helper()
	if summary.Moved != moved {
		t.Fatalf("summary = %#v", summary)
	}
	if targetNode != "" && summary.TargetNode != targetNode {
		t.Fatalf("summary = %#v", summary)
	}
}

func requireRunningMoveSummary(t *testing.T, summary task.AppOperationSummary, moved int) {
	t.Helper()
	if summary.Moved != moved || summary.Status != workloadmeta.AssignmentStatusRunning {
		t.Fatalf("summary = %#v", summary)
	}
}
