package placement_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/arcgolabs/collectionx/list"

	deployv1 "github.com/lyonbrown4d/orch/internal/deploy/v1alpha1"
	"github.com/lyonbrown4d/orch/internal/nodecapacity"
	"github.com/lyonbrown4d/orch/internal/placement"
)

// testSnapshotStore is a minimal in-test [nodecapacity.SnapshotStore] (not for production).
type testSnapshotStore struct {
	m map[string]nodecapacity.Snapshot
}

func newTestSnapshotStore() *testSnapshotStore {
	return &testSnapshotStore{m: make(map[string]nodecapacity.Snapshot)}
}

func (t *testSnapshotStore) Upsert(_ context.Context, snap nodecapacity.Snapshot) error {
	t.m[strings.TrimSpace(snap.NodeID)] = snap
	return nil
}

func (t *testSnapshotStore) Get(nodeID string) (nodecapacity.Snapshot, bool) {
	s, ok := t.m[strings.TrimSpace(nodeID)]
	return s, ok
}

func (t *testSnapshotStore) Len() int {
	return len(t.m)
}

func (t *testSnapshotStore) NodeIDs() *list.List[string] {
	out := list.NewListWithCapacity[string](len(t.m))
	for k := range t.m {
		out.Add(k)
	}
	out.Sort(strings.Compare)
	return out
}

func testCatalog(snaps ...nodecapacity.Snapshot) *nodecapacity.Catalog {
	mem := newTestSnapshotStore()
	ctx := context.Background()
	for _, s := range snaps {
		if err := mem.Upsert(ctx, s); err != nil {
			panic(err)
		}
	}
	return nodecapacity.NewCatalog(mem)
}

func TestChoose_prefersLowerCPU(t *testing.T) {
	t.Parallel()
	cat := testCatalog(
		nodecapacity.Snapshot{
			NodeID: "a", UpdatedAt: time.Now(), LogicalCPUCores: 4,
			CPUUsagePercent: 80, MemoryAvailBytes: 8 << 30,
		},
		nodecapacity.Snapshot{
			NodeID: "b", UpdatedAt: time.Now(), LogicalCPUCores: 4,
			CPUUsagePercent: 10, MemoryAvailBytes: 4 << 30,
		},
	)

	eng := placement.NewEngine()
	got, err := eng.Choose(context.Background(), deployv1.Workload{Name: "w"}, cat, "local")
	if err != nil {
		t.Fatal(err)
	}
	if got != "b" {
		t.Fatalf("got %q want b", got)
	}
}

func TestChoose_tieBreakByPreferredOrder(t *testing.T) {
	t.Parallel()
	cat := testCatalog(
		nodecapacity.Snapshot{
			NodeID: "x", UpdatedAt: time.Now(), LogicalCPUCores: 4,
			CPUUsagePercent: 40, MemoryAvailBytes: 8 << 30,
		},
		nodecapacity.Snapshot{
			NodeID: "y", UpdatedAt: time.Now(), LogicalCPUCores: 4,
			CPUUsagePercent: 40, MemoryAvailBytes: 8 << 30,
		},
	)

	w := deployv1.Workload{
		Name: "w",
		Scheduling: &deployv1.Scheduling{
			PreferredNodes: []string{"y", "x"},
		},
	}
	eng := placement.NewEngine()
	got, err := eng.Choose(context.Background(), w, cat, "local")
	if err != nil {
		t.Fatal(err)
	}
	if got != "y" {
		t.Fatalf("equal metrics: got %q want y (earlier in preferredNodes)", got)
	}
}

func TestChoose_respectsPreferredNodes(t *testing.T) {
	t.Parallel()
	cat := testCatalog(
		nodecapacity.Snapshot{
			NodeID: "hot", UpdatedAt: time.Now(), LogicalCPUCores: 8,
			CPUUsagePercent: 5, MemoryAvailBytes: 16 << 30,
		},
		nodecapacity.Snapshot{
			NodeID: "cold", UpdatedAt: time.Now(), LogicalCPUCores: 8,
			CPUUsagePercent: 50, MemoryAvailBytes: 16 << 30,
		},
	)

	w := deployv1.Workload{
		Name: "w",
		Scheduling: &deployv1.Scheduling{
			PreferredNodes: []string{"cold"},
		},
	}
	eng := placement.NewEngine()
	got, err := eng.Choose(context.Background(), w, cat, "local")
	if err != nil {
		t.Fatal(err)
	}
	if got != "cold" {
		t.Fatalf("got %q want cold", got)
	}
}
