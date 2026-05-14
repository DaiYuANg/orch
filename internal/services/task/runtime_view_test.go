package task_test

import (
	"context"
	"testing"
	"time"

	"github.com/lyonbrown4d/orch/internal/config"
	"github.com/lyonbrown4d/orch/internal/runtime/runtimeinfo"
	"github.com/lyonbrown4d/orch/internal/services/task"
	"github.com/lyonbrown4d/orch/internal/workerapi"
	"github.com/lyonbrown4d/orch/internal/workloadmeta"
)

func TestWorkloadRuntimeStatusDispatchesRemoteWorker(t *testing.T) {
	t.Parallel()

	statusCh := make(chan workerapi.WorkloadStatusBody, 1)
	logsCh := make(chan workerapi.WorkloadLogsBody, 1)
	worker := newInspectWorkerServer(t, statusCh, logsCh)
	cfg := config.Default()
	cfg.Cluster.Nodes = map[string]string{"node-b": worker.URL}
	harness := newTaskHarness(t, cfg, task.NewHTTPWorkerDispatcher(cfg))
	app := deployApp("remote-status", dockerWorkload("worker", "busybox", workloadPreferred("node-b")))
	harness.applyApp(t, app)
	harness.applyWorkerAssignment(t, app, "node-b", workloadmeta.AssignmentStatusRunning)

	status, err := harness.svc.WorkloadRuntimeStatus(context.Background(), app.Metadata, "worker")
	if err != nil {
		t.Fatal(err)
	}
	got := waitWorkerStatus(t, statusCh, 3*time.Second)
	requireWorkerInspectRequest(t, got.Node, got.Workload.Name, "node-b", "worker")
	if status.Status != "running" || status.NativeID != "remote-native-id" {
		t.Fatalf("status = %#v", status)
	}
}

func TestWorkloadRuntimeLogsDispatchesRemoteWorker(t *testing.T) {
	t.Parallel()

	statusCh := make(chan workerapi.WorkloadStatusBody, 1)
	logsCh := make(chan workerapi.WorkloadLogsBody, 1)
	worker := newInspectWorkerServer(t, statusCh, logsCh)
	cfg := config.Default()
	cfg.Cluster.Nodes = map[string]string{"node-b": worker.URL}
	harness := newTaskHarness(t, cfg, task.NewHTTPWorkerDispatcher(cfg))
	app := deployApp("remote-logs", dockerWorkload("worker", "busybox", workloadPreferred("node-b")))
	harness.applyApp(t, app)
	harness.applyWorkerAssignment(t, app, "node-b", workloadmeta.AssignmentStatusRunning)

	logs, err := harness.svc.WorkloadRuntimeLogs(context.Background(), app.Metadata, "worker", runtimeinfo.LogOptions{Tail: 42})
	if err != nil {
		t.Fatal(err)
	}
	got := waitWorkerLogs(t, logsCh, 3*time.Second)
	requireWorkerInspectRequest(t, got.Node, got.Workload.Name, "node-b", "worker")
	if got.Tail != 42 {
		t.Fatalf("tail = %d, want 42", got.Tail)
	}
	if logs.Source != "remote-log" || logs.Content != "remote log line\n" {
		t.Fatalf("logs = %#v", logs)
	}
}

func requireWorkerInspectRequest(t *testing.T, gotNode, gotWorkload, wantNode, wantWorkload string) {
	t.Helper()
	if gotNode != wantNode || gotWorkload != wantWorkload {
		t.Fatalf("worker inspect request node=%q workload=%q", gotNode, gotWorkload)
	}
}
