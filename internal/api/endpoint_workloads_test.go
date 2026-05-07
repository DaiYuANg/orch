package api

import (
	"context"
	"io"
	"log/slog"
	"testing"

	"github.com/daiyuang/orch/internal/services/registry"
)

func TestWorkloadsEndpointHandleMapsPublicItems(t *testing.T) {
	t.Parallel()

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	reg := registry.NewService(logger)
	reg.Upsert(registry.WorkloadRecord{
		Name:    "web",
		Node:    "node-a",
		Runtime: "docker",
		Image:   "nginx:alpine",
		Status:  "running",
	})

	out, err := NewWorkloadsEndpoint(reg).handle(context.Background(), &EmptyInput{})
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Body.Items) != 1 {
		t.Fatalf("items = %#v", out.Body.Items)
	}
	got := out.Body.Items[0]
	if got.Name != "web" || got.Node != "node-a" || got.Runtime != "docker" || got.Image != "nginx:alpine" || got.Status != "running" {
		t.Fatalf("workload item = %#v", got)
	}
	if got.UpdatedAt.IsZero() {
		t.Fatalf("updatedAt was not mapped: %#v", got)
	}
}
