package task

import (
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/daiyuang/orch/internal/config"
	deployv1 "github.com/daiyuang/orch/internal/deploy/v1alpha1"
	"github.com/daiyuang/orch/internal/nodeid"
	"github.com/daiyuang/orch/internal/raftsvc"
	"github.com/daiyuang/orch/internal/workloadmeta"
)

func TestListAppsAggregatesAssignmentStatus(t *testing.T) {
	cfg := config.Default()
	cfg.Raft.Enabled = false
	raft := raftsvc.New(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)), nodeid.Local{Value: "node-a"})
	svc := &Service{raft: raft}
	app := deployv1.App{
		Metadata: deployv1.Metadata{Name: "demo", Namespace: "default"},
		Workloads: []deployv1.Workload{
			{
				Name:    "api",
				Kind:    deployv1.WorkloadKindService,
				Runtime: deployv1.RuntimeDocker,
				Run:     deployv1.RunSpec{Artifact: deployv1.ArtifactSpec{Image: "api"}},
			},
			{
				Name:    "worker",
				Kind:    deployv1.WorkloadKindWorker,
				Runtime: deployv1.RuntimeDocker,
				Run:     deployv1.RunSpec{Artifact: deployv1.ArtifactSpec{Image: "worker"}},
			},
		},
	}
	if err := raft.ApplyDeployApp(app); err != nil {
		t.Fatal(err)
	}

	pending, ok := svc.GetApp(app.Metadata)
	if !ok {
		t.Fatal("expected app")
	}
	if pending.Status != AppStatusPending || pending.Pending != 2 {
		t.Fatalf("pending app = %#v", pending)
	}
	if pending.DesiredGeneration == "" || pending.ObservedGeneration != "" {
		t.Fatalf("pending generations = desired:%q observed:%q", pending.DesiredGeneration, pending.ObservedGeneration)
	}

	now := time.Now().UTC()
	generation := AppGeneration(app)
	if err := raft.ApplyWorkloadAssignment(workloadmeta.Assignment{
		Key:        workloadmeta.AssignmentKey(app.Metadata, "api"),
		Metadata:   app.Metadata,
		Workload:   "api",
		Node:       "node-a",
		Runtime:    deployv1.RuntimeDocker,
		Artifact:   "api",
		Status:     workloadmeta.AssignmentStatusRunning,
		Generation: generation,
		UpdatedAt:  now,
	}); err != nil {
		t.Fatal(err)
	}
	if err := raft.ApplyWorkloadAssignment(workloadmeta.Assignment{
		Key:        workloadmeta.AssignmentKey(app.Metadata, "worker"),
		Metadata:   app.Metadata,
		Workload:   "worker",
		Node:       "node-a",
		Runtime:    deployv1.RuntimeDocker,
		Artifact:   "worker",
		Status:     workloadmeta.AssignmentStatusStopped,
		Generation: generation,
		UpdatedAt:  now.Add(time.Second),
	}); err != nil {
		t.Fatal(err)
	}

	partial, ok := svc.GetApp(app.Metadata)
	if !ok {
		t.Fatal("expected app")
	}
	if partial.Status != AppStatusPartial || partial.Running != 1 || partial.Stopped != 1 || !partial.LastTransitionAt.Equal(now.Add(time.Second)) {
		t.Fatalf("partial app = %#v", partial)
	}
	if partial.ObservedGeneration != partial.DesiredGeneration {
		t.Fatalf("partial generations = desired:%q observed:%q", partial.DesiredGeneration, partial.ObservedGeneration)
	}

	if err := raft.ApplyWorkloadAssignment(workloadmeta.Assignment{
		Key:        workloadmeta.AssignmentKey(app.Metadata, "worker"),
		Metadata:   app.Metadata,
		Workload:   "worker",
		Node:       "node-a",
		Runtime:    deployv1.RuntimeDocker,
		Artifact:   "worker",
		Status:     workloadmeta.AssignmentStatusRunning,
		Generation: generation,
		UpdatedAt:  now.Add(2 * time.Second),
	}); err != nil {
		t.Fatal(err)
	}
	apps := svc.ListApps()
	if apps.Len() != 1 {
		t.Fatalf("apps len = %d", apps.Len())
	}
	running, _ := apps.Get(0)
	if running.Status != AppStatusRunning || running.Running != 2 {
		t.Fatalf("running app = %#v", running)
	}
}

func TestListAppsTreatsStaleAssignmentsAsPending(t *testing.T) {
	cfg := config.Default()
	cfg.Raft.Enabled = false
	raft := raftsvc.New(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)), nodeid.Local{Value: "node-a"})
	svc := &Service{raft: raft}
	app := deployv1.App{
		Metadata: deployv1.Metadata{Name: "demo", Namespace: "default"},
		Workloads: []deployv1.Workload{{
			Name:    "api",
			Kind:    deployv1.WorkloadKindService,
			Runtime: deployv1.RuntimeDocker,
			Run:     deployv1.RunSpec{Artifact: deployv1.ArtifactSpec{Image: "api:v2"}},
		}},
	}
	if err := raft.ApplyDeployApp(app); err != nil {
		t.Fatal(err)
	}
	if err := raft.ApplyWorkloadAssignment(workloadmeta.Assignment{
		Key:        workloadmeta.AssignmentKey(app.Metadata, "api"),
		Metadata:   app.Metadata,
		Workload:   "api",
		Node:       "node-a",
		Runtime:    deployv1.RuntimeDocker,
		Artifact:   "api:v1",
		Status:     workloadmeta.AssignmentStatusRunning,
		Generation: "old",
		UpdatedAt:  time.Now().UTC(),
	}); err != nil {
		t.Fatal(err)
	}

	view, ok := svc.GetApp(app.Metadata)
	if !ok {
		t.Fatal("expected app")
	}
	if view.Status != AppStatusPending || view.Pending != 1 || view.ObservedGeneration != "" {
		t.Fatalf("stale app = %#v", view)
	}
	workload, ok := view.Workloads.Get(0)
	if !ok || workload.Status != AppStatusPending || workload.Generation != "old" {
		t.Fatalf("stale workload = %#v", workload)
	}
}

func TestListAppsTreatsMissingGenerationAsPending(t *testing.T) {
	cfg := config.Default()
	cfg.Raft.Enabled = false
	raft := raftsvc.New(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)), nodeid.Local{Value: "node-a"})
	svc := &Service{raft: raft}
	app := deployv1.App{
		Metadata: deployv1.Metadata{Name: "demo", Namespace: "default"},
		Workloads: []deployv1.Workload{{
			Name:    "api",
			Kind:    deployv1.WorkloadKindService,
			Runtime: deployv1.RuntimeDocker,
			Run:     deployv1.RunSpec{Artifact: deployv1.ArtifactSpec{Image: "api:v1"}},
		}},
	}
	if err := raft.ApplyDeployApp(app); err != nil {
		t.Fatal(err)
	}
	if err := raft.ApplyWorkloadAssignment(workloadmeta.Assignment{
		Key:       workloadmeta.AssignmentKey(app.Metadata, "api"),
		Metadata:  app.Metadata,
		Workload:  "api",
		Node:      "node-a",
		Runtime:   deployv1.RuntimeDocker,
		Artifact:  "api:v1",
		Status:    workloadmeta.AssignmentStatusRunning,
		UpdatedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatal(err)
	}

	view, ok := svc.GetApp(app.Metadata)
	if !ok {
		t.Fatal("expected app")
	}
	if view.Status != AppStatusPending || view.Pending != 1 || view.ObservedGeneration != "" {
		t.Fatalf("missing-generation app = %#v", view)
	}
	workload, ok := view.Workloads.Get(0)
	if !ok || workload.Status != AppStatusPending || workload.Generation != "" {
		t.Fatalf("missing-generation workload = %#v", workload)
	}
}
