package api

import (
	"context"
	"io"
	"log/slog"
	"testing"

	"github.com/daiyuang/orch/internal/config"
	deployv1 "github.com/daiyuang/orch/internal/deploy/v1alpha1"
	"github.com/daiyuang/orch/internal/nodeid"
	"github.com/daiyuang/orch/internal/raftsvc"
	"github.com/daiyuang/orch/internal/services/task"
	"github.com/daiyuang/orch/internal/workloadmeta"
)

func TestAssignmentsEndpointHandle(t *testing.T) {
	t.Parallel()

	cfg := config.Default()
	cfg.Raft.Enabled = false
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	raft := raftsvc.New(cfg, logger, nodeid.Local{Value: "node-a"})
	meta := deployv1.Metadata{Name: "demo", Namespace: "default"}
	if err := raft.ApplyWorkloadAssignment(workloadmeta.Assignment{
		Metadata: meta,
		Workload: "web",
		Node:     "node-a",
		Runtime:  deployv1.RuntimeDocker,
		Image:    "nginx",
		Status:   workloadmeta.AssignmentStatusRunning,
	}); err != nil {
		t.Fatal(err)
	}

	tasks := task.NewService(logger, nil, nil, nil, cfg, task.Bundle{Raft: raft})
	out, err := NewAssignmentsEndpoint(tasks).handle(context.Background(), &EmptyInput{})
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Body.Items) != 1 {
		t.Fatalf("items = %#v", out.Body.Items)
	}
	got := out.Body.Items[0]
	if got.Key != workloadmeta.AssignmentKey(meta, "web") || got.Node != "node-a" || got.Status != workloadmeta.AssignmentStatusRunning {
		t.Fatalf("assignment = %#v", got)
	}
}
