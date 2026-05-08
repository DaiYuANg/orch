package api_test

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/arcgolabs/httpx"
	"github.com/arcgolabs/httpx/adapter"
	adapterfiber "github.com/arcgolabs/httpx/adapter/fiber"
	"github.com/gofiber/fiber/v2"

	"github.com/daiyuang/orch/internal/api"
	"github.com/daiyuang/orch/internal/apiclient"
	"github.com/daiyuang/orch/internal/config"
	"github.com/daiyuang/orch/internal/deploy/loader"
	deployorch "github.com/daiyuang/orch/internal/deploy/orch"
	deployv1 "github.com/daiyuang/orch/internal/deploy/v1alpha1"
	"github.com/daiyuang/orch/internal/metrics"
	"github.com/daiyuang/orch/internal/nodecapacity"
	"github.com/daiyuang/orch/internal/nodeid"
	"github.com/daiyuang/orch/internal/observability"
	"github.com/daiyuang/orch/internal/placement"
	"github.com/daiyuang/orch/internal/raftsvc"
	orchruntime "github.com/daiyuang/orch/internal/runtime"
	"github.com/daiyuang/orch/internal/services/registry"
	"github.com/daiyuang/orch/internal/services/task"
	"github.com/daiyuang/orch/internal/workerapi"
	"github.com/daiyuang/orch/internal/workloadmeta"
)

type e2eRuntimeProvider struct {
	mu       sync.Mutex
	deployed []deployv1.Workload
}

func (p *e2eRuntimeProvider) Kind() deployv1.RuntimeKind {
	return deployv1.RuntimeDocker
}

func (p *e2eRuntimeProvider) Deploy(_ context.Context, _ deployv1.Metadata, workload deployv1.Workload) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.deployed = append(p.deployed, workload)
	return nil
}

func (p *e2eRuntimeProvider) Stop(_ context.Context, _ deployv1.Metadata, _ string) error {
	return nil
}

func (p *e2eRuntimeProvider) deployedCount() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.deployed)
}

func TestDeploySourceDispatchesWorkerAndExposesState(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	workerCh := make(chan workerapi.DeployWorkloadBody, 3)
	stopCh := make(chan workerapi.StopWorkloadBody, 3)
	worker := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case workerapi.PathV1WorkerDeploy:
			var in workerapi.DeployWorkloadBody
			if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
				t.Fatalf("decode worker request: %v", err)
			}
			workerCh <- in
			out := workerapi.DeployWorkloadOutput{}
			out.Body.Accepted = true
			out.Body.Node = in.Node
			out.Body.Status = workloadmeta.AssignmentStatusRunning
			out.Body.Workload = in.Workload.Name
			_ = json.NewEncoder(w).Encode(out.Body)
		case workerapi.PathV1WorkerStop:
			var in workerapi.StopWorkloadBody
			if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
				t.Fatalf("decode worker stop request: %v", err)
			}
			stopCh <- in
			out := workerapi.StopWorkloadOutput{}
			out.Body.Accepted = true
			out.Body.Node = in.Node
			out.Body.Status = workloadmeta.AssignmentStatusStopped
			out.Body.Workload = in.Workload.Name
			_ = json.NewEncoder(w).Encode(out.Body)
		default:
			t.Fatalf("worker path = %q", r.URL.Path)
		}
	}))
	t.Cleanup(worker.Close)

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	cfg := config.Default()
	cfg.Raft.Enabled = false
	cfg.Auth.Enabled = false
	cfg.Cluster.Nodes = map[string]string{"node-b": worker.URL}
	cfg.Observability.Prometheus.Enabled = false
	cfg.Observability.OTLP.Enabled = false

	local := nodeid.Local{Value: "node-a"}
	raft := raftsvc.New(cfg, logger, local)
	store := raftsvc.NewRaftCapacityStore(raft)
	catalog := nodecapacity.NewCatalog(store)
	if err := store.Upsert(ctx, nodecapacity.Snapshot{
		NodeID:           "node-b",
		UpdatedAt:        time.Now(),
		LogicalCPUCores:  8,
		CPUUsagePercent:  5,
		MemoryAvailBytes: 16 << 30,
	}); err != nil {
		t.Fatal(err)
	}

	obs, err := observability.New(cfg, nil, logger)
	if err != nil {
		t.Fatal(err)
	}
	metricsSvc := metrics.New(obs)
	localRuntime := &e2eRuntimeProvider{}
	runtimeManager := orchruntime.NewManager(logger, localRuntime)
	registrySvc := registry.NewService(logger)
	taskSvc := task.NewService(logger, metricsSvc, runtimeManager, registrySvc, cfg, task.Bundle{
		LocalNode:  local,
		Catalog:    catalog,
		Placement:  placement.NewEngine(),
		Raft:       raft,
		Dispatcher: task.NewHTTPWorkerDispatcher(cfg),
	})
	taskSvc.StartDeployReconcile(ctx)

	loaderSvc := newE2ELoader(t)
	fiberApp, rt := newE2EServerRuntime(logger)
	api.Register(rt, cfg, registrySvc, taskSvc, loaderSvc, nil, raft)
	baseURL := startTestFiberServer(t, fiberApp)
	client, err := apiclient.New(baseURL, "")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := client.Close(); err != nil {
			t.Logf("close client: %v", err)
		}
	})

	raftStatus, err := client.RaftStatus(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if raftStatus.Body.Enabled || raftStatus.Body.Ready || raftStatus.Body.State != "disabled" {
		t.Fatalf("raft status = %#v", raftStatus.Body)
	}

	out, err := client.DeploySource(ctx, "app.yaml", `metadata:
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
`)
	if err != nil {
		t.Fatal(err)
	}
	if !out.Body.Accepted || out.Body.App != "e2e-demo" || out.Body.Workloads != 1 {
		t.Fatalf("deploy response = %#v", out.Body)
	}

	select {
	case got := <-workerCh:
		if got.Node != "node-b" || got.Workload.Name != "worker" || got.Workload.Run.Artifact.Image != "busybox" {
			t.Fatalf("worker request = %#v", got)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for worker dispatch")
	}
	if localRuntime.deployedCount() != 0 {
		t.Fatalf("local runtime should not deploy remote workload, got %d deploys", localRuntime.deployedCount())
	}

	assignment := waitHTTPAssignment(t, ctx, client, "default/e2e-demo/worker", "node-b", workloadmeta.AssignmentStatusRunning)
	if assignment.Node != "node-b" || assignment.Status != workloadmeta.AssignmentStatusRunning || assignment.Artifact != "busybox" {
		t.Fatalf("assignment = %#v", assignment)
	}

	workload := waitHTTPWorkload(t, ctx, client, "worker")
	if workload.Node != "node-b" || workload.Status != workloadmeta.AssignmentStatusRunning || workload.Artifact != "busybox" {
		t.Fatalf("workload = %#v", workload)
	}
	appStatus := waitHTTPApp(t, ctx, client, "default", "e2e-demo", workloadmeta.AssignmentStatusRunning)
	if appStatus.Running != 1 || appStatus.DesiredWorkloads != 1 || appStatus.Workloads.Len() != 1 {
		t.Fatalf("app status = %#v", appStatus)
	}

	stopped, err := client.StopDeploy(ctx, "default", "e2e-demo")
	if err != nil {
		t.Fatal(err)
	}
	if !stopped.Body.Accepted || stopped.Body.App != "e2e-demo" || stopped.Body.Status != workloadmeta.AssignmentStatusStopped {
		t.Fatalf("stop response = %#v", stopped.Body)
	}
	select {
	case got := <-stopCh:
		if got.Node != "node-b" || got.Workload.Name != "worker" || got.Metadata.Name != "e2e-demo" {
			t.Fatalf("worker stop request = %#v", got)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for worker stop")
	}
	stoppedAssignment := waitHTTPAssignment(t, ctx, client, "default/e2e-demo/worker", "node-b", workloadmeta.AssignmentStatusStopped)
	if stoppedAssignment.Status != workloadmeta.AssignmentStatusStopped {
		t.Fatalf("stopped assignment = %#v", stoppedAssignment)
	}
	waitHTTPWorkloadGone(t, ctx, client, "worker")
	waitHTTPApp(t, ctx, client, "default", "e2e-demo", workloadmeta.AssignmentStatusStopped)

	started, err := client.StartDeploy(ctx, "default", "e2e-demo")
	if err != nil {
		t.Fatal(err)
	}
	if !started.Body.Accepted || started.Body.App != "e2e-demo" || started.Body.Status != workloadmeta.AssignmentStatusRunning {
		t.Fatalf("start response = %#v", started.Body)
	}
	select {
	case got := <-workerCh:
		if got.Node != "node-b" || got.Workload.Name != "worker" || got.Workload.Run.Artifact.Image != "busybox" {
			t.Fatalf("worker start request = %#v", got)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for worker start")
	}
	waitHTTPAssignment(t, ctx, client, "default/e2e-demo/worker", "node-b", workloadmeta.AssignmentStatusRunning)
	waitHTTPWorkload(t, ctx, client, "worker")
	waitHTTPApp(t, ctx, client, "default", "e2e-demo", workloadmeta.AssignmentStatusRunning)

	restarted, err := client.RestartDeploy(ctx, "default", "e2e-demo")
	if err != nil {
		t.Fatal(err)
	}
	if !restarted.Body.Accepted || restarted.Body.App != "e2e-demo" || restarted.Body.Status != workloadmeta.AssignmentStatusRunning {
		t.Fatalf("restart response = %#v", restarted.Body)
	}
	select {
	case got := <-stopCh:
		if got.Node != "node-b" || got.Workload.Name != "worker" || got.Metadata.Name != "e2e-demo" {
			t.Fatalf("worker restart stop request = %#v", got)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for worker restart stop")
	}
	select {
	case got := <-workerCh:
		if got.Node != "node-b" || got.Workload.Name != "worker" || got.Workload.Run.Artifact.Image != "busybox" {
			t.Fatalf("worker restart start request = %#v", got)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for worker restart start")
	}
	waitHTTPAssignment(t, ctx, client, "default/e2e-demo/worker", "node-b", workloadmeta.AssignmentStatusRunning)
	waitHTTPWorkload(t, ctx, client, "worker")
	waitHTTPApp(t, ctx, client, "default", "e2e-demo", workloadmeta.AssignmentStatusRunning)

	deleted, err := client.DeleteDeploy(ctx, "default", "e2e-demo")
	if err != nil {
		t.Fatal(err)
	}
	if !deleted.Body.Accepted || deleted.Body.App != "e2e-demo" || deleted.Body.Status != workloadmeta.AssignmentStatusStopped {
		t.Fatalf("delete response = %#v", deleted.Body)
	}
	select {
	case got := <-stopCh:
		if got.Node != "node-b" || got.Workload.Name != "worker" || got.Metadata.Name != "e2e-demo" {
			t.Fatalf("worker delete stop request = %#v", got)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for worker delete stop")
	}
	deletedAssignment := waitHTTPAssignment(t, ctx, client, "default/e2e-demo/worker", "node-b", workloadmeta.AssignmentStatusStopped)
	if deletedAssignment.Status != workloadmeta.AssignmentStatusStopped {
		t.Fatalf("deleted assignment = %#v", deletedAssignment)
	}
	waitHTTPWorkloadGone(t, ctx, client, "worker")
	waitHTTPAppGone(t, ctx, client, "default", "e2e-demo")
}

func newE2ELoader(t *testing.T) *loader.Loader {
	t.Helper()
	compiler, err := deployorch.NewCompiler()
	if err != nil {
		t.Fatal(err)
	}
	orchLoader, err := deployorch.NewOrch(compiler)
	if err != nil {
		t.Fatal(err)
	}
	loaderSvc, err := loader.NewLoader(orchLoader)
	if err != nil {
		t.Fatal(err)
	}
	return loaderSvc
}

func newE2EServerRuntime(logger *slog.Logger) (*fiber.App, httpx.ServerRuntime) {
	fiberApp := fiber.New(fiber.Config{DisableStartupMessage: true})
	fiberAdapter := adapterfiber.New(fiberApp, adapter.HumaOptions{
		Title:       "orch API test",
		Version:     "test",
		Description: "orch API test",
	})
	rt := httpx.New(
		httpx.WithAdapter(fiberAdapter),
		httpx.WithLogger(logger),
		httpx.WithValidation(),
		httpx.WithBasePath("/api"),
	)
	return fiberApp, rt
}

func startTestFiberServer(t *testing.T, app *fiber.App) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	errCh := make(chan error, 1)
	go func() {
		errCh <- app.Listener(ln)
	}()
	t.Cleanup(func() {
		if err := app.Shutdown(); err != nil {
			t.Logf("shutdown fiber app: %v", err)
		}
		select {
		case err := <-errCh:
			if err != nil {
				t.Logf("fiber listener stopped: %v", err)
			}
		case <-time.After(2 * time.Second):
			t.Log("timed out waiting for fiber listener shutdown")
		}
	})
	return "http://" + ln.Addr().String()
}

func waitHTTPAssignment(t *testing.T, ctx context.Context, client *apiclient.Client, key, node, status string) api.AssignmentItem {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		out, err := client.ListAssignments(ctx)
		if err == nil {
			var found api.AssignmentItem
			ok := false
			out.Body.Items.Range(func(_ int, item api.AssignmentItem) bool {
				if item.Key == key && item.Node == node && item.Status == status {
					found = item
					ok = true
					return false
				}
				return true
			})
			if ok {
				return found
			}
		}
		if time.Now().After(deadline) {
			t.Fatalf("assignment %q did not converge to node=%q status=%q", key, node, status)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func waitHTTPWorkload(t *testing.T, ctx context.Context, client *apiclient.Client, name string) api.WorkloadItem {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		out, err := client.ListWorkloads(ctx)
		if err == nil {
			var found api.WorkloadItem
			ok := false
			out.Body.Items.Range(func(_ int, item api.WorkloadItem) bool {
				if item.Name == name {
					found = item
					ok = true
					return false
				}
				return true
			})
			if ok {
				return found
			}
		}
		if time.Now().After(deadline) {
			t.Fatalf("workload %q did not appear", name)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func waitHTTPWorkloadGone(t *testing.T, ctx context.Context, client *apiclient.Client, name string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		out, err := client.ListWorkloads(ctx)
		if err == nil {
			found := false
			out.Body.Items.Range(func(_ int, item api.WorkloadItem) bool {
				if item.Name == name {
					found = true
					return false
				}
				return true
			})
			if !found {
				return
			}
		}
		if time.Now().After(deadline) {
			t.Fatalf("workload %q still present", name)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func waitHTTPApp(t *testing.T, ctx context.Context, client *apiclient.Client, namespace, name, status string) api.AppDetailItem {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		out, err := client.GetApp(ctx, namespace, name)
		if err == nil && out.Body.Status == status {
			return out.Body
		}
		if time.Now().After(deadline) {
			t.Fatalf("app %s/%s did not converge to status=%q", namespace, name, status)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func waitHTTPAppGone(t *testing.T, ctx context.Context, client *apiclient.Client, namespace, name string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		out, err := client.ListApps(ctx)
		if err == nil {
			found := false
			out.Body.Items.Range(func(_ int, item api.AppItem) bool {
				if item.Namespace == namespace && item.Name == name {
					found = true
					return false
				}
				return true
			})
			if !found {
				return
			}
		}
		if time.Now().After(deadline) {
			t.Fatalf("app %s/%s still present", namespace, name)
		}
		time.Sleep(20 * time.Millisecond)
	}
}
