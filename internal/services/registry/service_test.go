package registry_test

import (
	"log/slog"
	"testing"

	"github.com/lyonbrown4d/orch/internal/services/registry"
)

func TestService_List_sortedByName(t *testing.T) {
	t.Parallel()
	s := registry.NewService(slog.Default())
	s.Upsert(registry.WorkloadRecord{Name: "zebra", Runtime: "docker", Artifact: "z", Status: "up"})
	s.Upsert(registry.WorkloadRecord{Name: "alpha", Runtime: "docker", Artifact: "a", Status: "up"})
	s.Upsert(registry.WorkloadRecord{Name: "mule", Runtime: "docker", Artifact: "m", Status: "up"})

	got := s.List()
	if got.Len() != 3 {
		t.Fatalf("len=%d want 3", got.Len())
	}
	first, _ := got.Get(0)
	second, _ := got.Get(1)
	third, _ := got.Get(2)
	if first.Name != "alpha" || second.Name != "mule" || third.Name != "zebra" {
		t.Fatalf("order=%v want alpha,mule,zebra", []string{first.Name, second.Name, third.Name})
	}
}
