package api_test

import (
	"context"
	"log/slog"
	"testing"

	"github.com/daiyuang/orch/internal/api"
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
	logger := slog.New(slog.DiscardHandler)
	raft := raftsvc.New(cfg, logger, nodeid.Local{Value: "node-a"})
	meta := deployv1.Metadata{Name: "demo", Namespace: "default"}
	if err := raft.ApplyWorkloadAssignment(context.Background(), workloadmeta.Assignment{
		Metadata: meta,
		Workload: "web",
		Node:     "node-a",
		Runtime:  deployv1.RuntimeDocker,
		Artifact: "nginx",
		Status:   workloadmeta.AssignmentStatusRunning,
	}); err != nil {
		t.Fatal(err)
	}

	tasks := task.NewService(logger, nil, nil, nil, cfg, task.Bundle{Raft: raft})
	out, err := api.NewAssignmentsEndpoint(tasks).Handle(context.Background(), &api.EmptyInput{})
	if err != nil {
		t.Fatal(err)
	}
	if out.Body.Items.Len() != 1 {
		t.Fatalf("items = %#v", out.Body.Items)
	}
	got, ok := out.Body.Items.Get(0)
	if !ok {
		t.Fatal("missing assignment item")
	}
	if got.Key != workloadmeta.AssignmentKey(meta, "web") || got.Node != "node-a" || got.Status != workloadmeta.AssignmentStatusRunning {
		t.Fatalf("assignment = %#v", got)
	}
}
