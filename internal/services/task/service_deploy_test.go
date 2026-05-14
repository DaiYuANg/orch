package task_test

import (
	"testing"
	"time"

	"github.com/daiyuang/orch/internal/config"
	deployv1 "github.com/daiyuang/orch/internal/deploy/v1alpha1"
	"github.com/daiyuang/orch/internal/services/task"
	"github.com/daiyuang/orch/internal/workerapi"
	"github.com/daiyuang/orch/internal/workloadmeta"
)

func TestSubmitDeployReconcilesThroughPlacementAndRuntime(t *testing.T) {
	t.Parallel()

	harness := newTaskHarness(t, config.Default(), nil)
	harness.startReconcile()
	app := deployApp("demo", dockerWorkload("web", "nginx",
		workloadKind(deployv1.WorkloadKindService),
		workloadResources(1, 1),
		workloadPreferred("node-a"),
	))

	harness.submitDeploy(t, app)
	got := harness.waitRuntimeDeploy(t, deployReconcileTimeout)
	requireWorkloadName(t, got, "web")
	harness.requireRegistryRecord(t, "web", "node-a", "running", deployReconcileTimeout)
	requireLocalCapacitySnapshot(t, harness)
	assignment := harness.requireAssignment(t, app, "web", "node-a", workloadmeta.AssignmentStatusRunning)
	requireAssignmentPayload(t, assignment, deployv1.RuntimeDocker, "nginx")
}

func TestSubmitDeployReconcilesWorkloadsInDependencyOrder(t *testing.T) {
	t.Parallel()

	harness := newTaskHarness(t, config.Default(), nil)
	harness.startReconcile()
	app := deployApp("ordered-demo",
		dockerWorkload("api", "api", workloadKind(deployv1.WorkloadKindService), workloadDependsOn("db")),
		dockerWorkload("db", "postgres", workloadKind(deployv1.WorkloadKindStateful)),
	)

	harness.submitDeploy(t, app)
	first := harness.waitRuntimeDeploy(t, deployReconcileTimeout)
	second := harness.waitRuntimeDeploy(t, deployReconcileTimeout)
	if first.Name != "db" || second.Name != "api" {
		t.Fatalf("deploy order = %q, %q; want db, api", first.Name, second.Name)
	}
}

func TestSubmitDeployDispatchesRemoteWorker(t *testing.T) {
	t.Parallel()

	dispatchCh := make(chan workerapi.DeployWorkloadBody, 1)
	worker := newDeployWorkerServer(t, dispatchCh, "running")
	cfg := config.Default()
	cfg.Cluster.Nodes = map[string]string{"node-b": worker.URL}
	harness := newTaskHarness(t, cfg, task.NewHTTPWorkerDispatcher(cfg))
	harness.seedRemoteCapacity(t, "node-b")
	harness.startReconcile()
	app := deployApp("remote-demo", dockerWorkload("worker", "busybox", workloadPreferred("node-b")))

	harness.submitDeploy(t, app)
	requireWorkerDispatch(t, waitWorkerDispatch(t, dispatchCh, 3*time.Second), "node-b", "worker")
	harness.requireNoLocalDeploy(t)
	harness.requireRegistryRecord(t, "worker", "node-b", "running", 3*time.Second)
	assignment := harness.requireAssignment(t, app, "worker", "node-b", workloadmeta.AssignmentStatusRunning)
	requireAssignmentPayload(t, assignment, deployv1.RuntimeDocker, "busybox")
}

func TestSubmitDeployDispatchesConfiguredPreferredWorkerWithoutCapacitySnapshot(t *testing.T) {
	t.Parallel()

	dispatchCh := make(chan workerapi.DeployWorkloadBody, 1)
	worker := newDeployWorkerServer(t, dispatchCh, "running")
	cfg := config.Default()
	cfg.Cluster.Nodes = map[string]string{"node-b": worker.URL}
	harness := newTaskHarness(t, cfg, task.NewHTTPWorkerDispatcher(cfg))
	harness.startReconcile()
	app := deployApp("remote-no-capacity", dockerWorkload("worker", "busybox", workloadPreferred("node-b")))

	harness.submitDeploy(t, app)
	requireWorkerDispatch(t, waitWorkerDispatch(t, dispatchCh, 3*time.Second), "node-b", "worker")
	harness.requireNoLocalDeploy(t)
	assignment := harness.requireAssignment(t, app, "worker", "node-b", workloadmeta.AssignmentStatusRunning)
	requireNonEmptyGeneration(t, assignment)
}

func TestSubmitDeployRecordsFailedAssignmentOnRemoteDispatchError(t *testing.T) {
	t.Parallel()

	dispatchCh := make(chan struct{}, 4)
	worker := newFailingDeployWorkerServer(t, dispatchCh)
	cfg := config.Default()
	cfg.Cluster.Nodes = map[string]string{"node-b": worker.URL}
	harness := newTaskHarness(t, cfg, task.NewHTTPWorkerDispatcher(cfg))
	harness.seedRemoteCapacity(t, "node-b")
	harness.startReconcile()
	app := deployApp("remote-fail", dockerWorkload("worker", "busybox", workloadPreferred("node-b")))

	harness.submitDeploy(t, app)
	waitDispatchSignal(t, dispatchCh, 3*time.Second)
	assignment := harness.requireAssignment(t, app, "worker", "node-b", workloadmeta.AssignmentStatusFailed)
	requireAssignmentError(t, assignment)
}

func requireWorkloadName(t *testing.T, got deployv1.Workload, want string) {
	t.Helper()
	if got.Name != want {
		t.Fatalf("deployed workload = %q, want %s", got.Name, want)
	}
}

func requireLocalCapacitySnapshot(t *testing.T, harness *taskHarness) {
	t.Helper()
	if harness.catalog.Len() == 0 {
		t.Fatal("expected local capacity snapshot to be recorded for placement")
	}
}

func requireNonEmptyGeneration(t *testing.T, assignment workloadmeta.Assignment) {
	t.Helper()
	if assignment.Generation == "" {
		t.Fatalf("assignment missing generation: %#v", assignment)
	}
}

func requireAssignmentError(t *testing.T, assignment workloadmeta.Assignment) {
	t.Helper()
	if assignment.Error == "" {
		t.Fatalf("expected assignment error, got %#v", assignment)
	}
}
