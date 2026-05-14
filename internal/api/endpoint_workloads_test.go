package api_test

import (
	"context"
	"log/slog"
	"testing"

	"github.com/lyonbrown4d/orch/internal/api"
	"github.com/lyonbrown4d/orch/internal/services/registry"
)

func TestWorkloadsEndpointHandleMapsPublicItems(t *testing.T) {
	t.Parallel()

	logger := slog.New(slog.DiscardHandler)
	reg := registry.NewService(logger)
	reg.Upsert(registry.WorkloadRecord{
		Name:     "web",
		Node:     "node-a",
		Runtime:  "docker",
		Artifact: "nginx:alpine",
		Status:   "running",
	})

	out, err := api.NewWorkloadsEndpoint(reg).Handle(context.Background(), &api.EmptyInput{})
	if err != nil {
		t.Fatal(err)
	}
	if out.Body.Items.Len() != 1 {
		t.Fatalf("items = %#v", out.Body.Items)
	}
	got, ok := out.Body.Items.Get(0)
	if !ok {
		t.Fatal("missing workload item")
	}
	if got.Name != "web" || got.Node != "node-a" || got.Runtime != "docker" || got.Artifact != "nginx:alpine" || got.Status != "running" {
		t.Fatalf("workload item = %#v", got)
	}
	if got.UpdatedAt.IsZero() {
		t.Fatalf("updatedAt was not mapped: %#v", got)
	}
}
