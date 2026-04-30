package registry

import (
	"log/slog"
	"testing"
)

func TestService_List_sortedByName(t *testing.T) {
	t.Parallel()
	s := NewService(slog.Default())
	s.Upsert(WorkloadRecord{Name: "zebra", Runtime: "docker", Image: "z", Status: "up"})
	s.Upsert(WorkloadRecord{Name: "alpha", Runtime: "docker", Image: "a", Status: "up"})
	s.Upsert(WorkloadRecord{Name: "mule", Runtime: "docker", Image: "m", Status: "up"})

	got := s.List()
	if len(got) != 3 {
		t.Fatalf("len=%d want 3", len(got))
	}
	if got[0].Name != "alpha" || got[1].Name != "mule" || got[2].Name != "zebra" {
		t.Fatalf("order=%v want alpha,mule,zebra", []string{got[0].Name, got[1].Name, got[2].Name})
	}
}
