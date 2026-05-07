package placement

import (
	"context"
	"fmt"
	"strings"

	"github.com/arcgolabs/collectionx/list"
	"github.com/arcgolabs/collectionx/set"
	deployv1 "github.com/daiyuang/orch/internal/deploy/v1alpha1"
	"github.com/daiyuang/orch/internal/nodecapacity"
	"github.com/samber/mo"
)

// chooseV1Alpha1 implements v1alpha1: among feasible nodes (memory + projected CPU millicores),
// pick lowest CPUUsagePercent, tie-break larger MemoryAvailBytes, then earlier scheduling.preferredNodes entry.
func chooseV1Alpha1(ctx context.Context, w deployv1.Workload, catalog *nodecapacity.Catalog, localNodeID string) (string, error) {
	_ = ctx
	if catalog == nil || catalog.Len() == 0 {
		id := strings.TrimSpace(localNodeID)
		if id == "" {
			id = "local"
		}
		return id, nil
	}

	candidates := catalog.NodeIDs()
	want, restrictPreferred := preferredNames(w)
	if restrictPreferred {
		if want.IsEmpty() {
			return "", fmt.Errorf("placement: scheduling.preferredNodes has no valid names")
		}
		candidates = list.FilterList(list.NewList(candidates...), func(_ int, id string) bool {
			return want.Contains(strings.TrimSpace(id))
		}).Values()
		if len(candidates) == 0 {
			return "", fmt.Errorf("placement: no catalog nodes match scheduling.preferredNodes")
		}
	}

	var reqCPU, reqMem int64
	if w.Resources != nil {
		reqCPU = w.Resources.CPUMillis
		reqMem = w.Resources.MemoryBytes
	}

	var best mo.Option[scoredNode]
	for _, id := range candidates {
		snap, ok := catalog.Get(id)
		if !ok {
			continue
		}
		if !feasible(reqCPU, reqMem, snap) {
			continue
		}
		cur := scoredNode{id: id, snap: snap}
		if best.IsAbsent() {
			best = mo.Some(cur)
			continue
		}
		prev, _ := best.Get()
		if better(cur.snap, prev.snap) {
			best = mo.Some(cur)
			continue
		}
		if restrictPreferred && snapEqual(cur.snap, prev.snap) {
			if preferredRank(want, cur.id) < preferredRank(want, prev.id) {
				best = mo.Some(cur)
			}
		}
	}

	v, ok := best.Get()
	if !ok {
		return "", fmt.Errorf("placement: no feasible node for workload %q (resources / stale catalog)", w.Name)
	}
	return v.id, nil
}

type scoredNode struct {
	id   string
	snap nodecapacity.Snapshot
}

// preferredNames returns the normalized preferred-node names in YAML/list order (deduplicated) and
// whether scheduling restricts placement.
func preferredNames(w deployv1.Workload) (*set.OrderedSet[string], bool) {
	if w.Scheduling == nil || len(w.Scheduling.PreferredNodes) == 0 {
		return nil, false
	}
	raw := w.PreferredNodeList()
	s := set.NewOrderedSetWithCapacity[string](raw.Len())
	raw.Range(func(_ int, n string) bool {
		n = strings.TrimSpace(n)
		if n != "" {
			s.Add(n)
		}
		return true
	})
	return s, true
}

func snapEqual(a, b nodecapacity.Snapshot) bool {
	return a.CPUUsagePercent == b.CPUUsagePercent && a.MemoryAvailBytes == b.MemoryAvailBytes
}

// preferredRank is the zero-based index in the manifest preferred list (dedupe order); missing ids sort last.
func preferredRank(want *set.OrderedSet[string], nodeID string) int {
	if want == nil {
		return 0
	}
	vals := want.Values()
	for i, id := range vals {
		if id == nodeID {
			return i
		}
	}
	return len(vals) + 1
}

func feasible(reqCPUmillis, reqMemBytes int64, s nodecapacity.Snapshot) bool {
	if reqMemBytes > 0 && int64(s.MemoryAvailBytes) < reqMemBytes {
		return false
	}
	if reqCPUmillis <= 0 {
		return true
	}
	remaining := estimatedCPUmillisRemaining(s)
	return remaining >= reqCPUmillis
}

// estimatedCPUmillisRemaining treats CPUMillis like Kubernetes millicores (1000 per core).
func estimatedCPUmillisRemaining(s nodecapacity.Snapshot) int64 {
	if s.LogicalCPUCores <= 0 {
		return 0
	}
	total := int64(s.LogicalCPUCores) * 1000
	if s.CPUUsagePercent <= 0 {
		return total
	}
	p := s.CPUUsagePercent
	if p > 100 {
		p = 100
	}
	availFrac := (100.0 - float64(p)) / 100.0
	return int64(float64(total) * availFrac)
}

func better(a, b nodecapacity.Snapshot) bool {
	if a.CPUUsagePercent != b.CPUUsagePercent {
		return a.CPUUsagePercent < b.CPUUsagePercent
	}
	return a.MemoryAvailBytes > b.MemoryAvailBytes
}
