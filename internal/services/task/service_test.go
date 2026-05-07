package task

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/daiyuang/orch/internal/config"
	deployv1 "github.com/daiyuang/orch/internal/deploy/v1alpha1"
	"github.com/daiyuang/orch/internal/metrics"
	"github.com/daiyuang/orch/internal/nodecapacity"
	"github.com/daiyuang/orch/internal/nodeid"
	"github.com/daiyuang/orch/internal/observability"
	"github.com/daiyuang/orch/internal/placement"
	"github.com/daiyuang/orch/internal/raftsvc"
	orchruntime "github.com/daiyuang/orch/internal/runtime"
	"github.com/daiyuang/orch/internal/services/registry"
	"github.com/daiyuang/orch/internal/workerapi"
)

type fakeRuntimeProvider struct {
	mu       sync.Mutex
	deployed []deployv1.Workload
	ch       chan deployv1.Workload
}

func newFakeRuntimeProvider() *fakeRuntimeProvider {
	return &fakeRuntimeProvider{ch: make(chan deployv1.Workload, 4)}
}

func (p *fakeRuntimeProvider) Kind() deployv1.RuntimeKind {
	return deployv1.RuntimeDocker
}

func (p *fakeRuntimeProvider) Deploy(_ context.Context, _ deployv1.Metadata, workload deployv1.Workload) error {
	p.mu.Lock()
	p.deployed = append(p.deployed, workload)
	p.mu.Unlock()
	p.ch <- workload
	return nil
}

func (p *fakeRuntimeProvider) Stop(_ context.Context, _ deployv1.Metadata, _ string) error {
	return nil
}

func newTestMetrics(t *testing.T, cfg config.Config, logger *slog.Logger) *metrics.Service {
	t.Helper()
	cfg.Observability.Prometheus.Enabled = false
	cfg.Observability.OTLP.Enabled = false
	obs, err := observability.New(cfg, nil, logger)
	if err != nil {
		t.Fatal(err)
	}
	return metrics.New(obs)
}

func TestSubmitDeployReconcilesThroughPlacementAndRuntime(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	cfg := config.Default()
	cfg.Raft.Enabled = false
	local := nodeid.Local{Value: "node-a"}
	raft := raftsvc.New(cfg, logger, local)
	catalog := nodecapacity.NewCatalog(raftsvc.NewRaftCapacityStore(raft))
	fakeRuntime := newFakeRuntimeProvider()
	runtimeManager := orchruntime.NewManager(logger, fakeRuntime)
	registrySvc := registry.NewService(logger)
	svc := NewService(logger, newTestMetrics(t, cfg, logger), runtimeManager, registrySvc, cfg, Bundle{
		LocalNode: local,
		Catalog:   catalog,
		Placement: placement.NewEngine(),
		Raft:      raft,
	})

	svc.StartDeployReconcile(ctx)

	app := &deployv1.App{
		Metadata: deployv1.Metadata{Name: "demo", Namespace: "default"},
		Workloads: []deployv1.Workload{
			{
				Name:    "web",
				Kind:    deployv1.WorkloadKindService,
				Runtime: deployv1.RuntimeDocker,
				Run: deployv1.RunSpec{
					Image: "nginx",
				},
				Resources: &deployv1.Resources{
					CPUMillis:   1,
					MemoryBytes: 1,
				},
				Scheduling: &deployv1.Scheduling{
					PreferredNodes: []string{"node-a"},
				},
			},
		},
	}

	if err := svc.SubmitDeploy(ctx, app); err != nil {
		t.Fatal(err)
	}

	select {
	case got := <-fakeRuntime.ch:
		if got.Name != "web" {
			t.Fatalf("deployed workload = %q, want web", got.Name)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for runtime deploy")
	}

	deadline := time.Now().Add(3 * time.Second)
	for {
		items := registrySvc.List()
		if len(items) == 1 && items[0].Name == "web" && items[0].Node == "node-a" && items[0].Status == "running" {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("registry did not converge, got %#v", items)
		}
		time.Sleep(10 * time.Millisecond)
	}

	if catalog.Len() == 0 {
		t.Fatal("expected local capacity snapshot to be recorded for placement")
	}
}

func TestSubmitDeployDispatchesRemoteWorker(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	dispatchCh := make(chan workerapi.DeployWorkloadInput, 1)
	worker := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != workerapi.PathV1WorkerDeploy {
			t.Fatalf("worker path = %q, want %q", r.URL.Path, workerapi.PathV1WorkerDeploy)
		}
		var in workerapi.DeployWorkloadInput
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
			t.Fatalf("decode worker request: %v", err)
		}
		dispatchCh <- in
		out := workerapi.DeployWorkloadOutput{}
		out.Body.Accepted = true
		out.Body.Node = in.Body.Node
		out.Body.Status = "running"
		out.Body.Workload = in.Body.Workload.Name
		_ = json.NewEncoder(w).Encode(out)
	}))
	t.Cleanup(worker.Close)

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	cfg := config.Default()
	cfg.Raft.Enabled = false
	cfg.Cluster.Nodes = map[string]string{"node-b": worker.URL}
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
	fakeRuntime := newFakeRuntimeProvider()
	runtimeManager := orchruntime.NewManager(logger, fakeRuntime)
	registrySvc := registry.NewService(logger)
	svc := NewService(logger, newTestMetrics(t, cfg, logger), runtimeManager, registrySvc, cfg, Bundle{
		LocalNode:  local,
		Catalog:    catalog,
		Placement:  placement.NewEngine(),
		Raft:       raft,
		Dispatcher: NewHTTPWorkerDispatcher(cfg),
	})

	svc.StartDeployReconcile(ctx)

	app := &deployv1.App{
		Metadata: deployv1.Metadata{Name: "remote-demo", Namespace: "default"},
		Workloads: []deployv1.Workload{
			{
				Name:    "worker",
				Kind:    deployv1.WorkloadKindWorker,
				Runtime: deployv1.RuntimeDocker,
				Run: deployv1.RunSpec{
					Image: "busybox",
				},
				Scheduling: &deployv1.Scheduling{
					PreferredNodes: []string{"node-b"},
				},
			},
		},
	}

	if err := svc.SubmitDeploy(ctx, app); err != nil {
		t.Fatal(err)
	}

	select {
	case got := <-dispatchCh:
		if got.Body.Node != "node-b" {
			t.Fatalf("dispatch node = %q, want node-b", got.Body.Node)
		}
		if got.Body.Workload.Name != "worker" {
			t.Fatalf("dispatch workload = %q, want worker", got.Body.Workload.Name)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for worker dispatch")
	}

	select {
	case got := <-fakeRuntime.ch:
		t.Fatalf("local runtime should not deploy remote workload, got %q", got.Name)
	default:
	}

	deadline := time.Now().Add(3 * time.Second)
	for {
		items := registrySvc.List()
		if len(items) == 1 && items[0].Name == "worker" && items[0].Node == "node-b" && items[0].Status == "running" {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("registry did not record remote running status, got %#v", items)
		}
		time.Sleep(10 * time.Millisecond)
	}
}
