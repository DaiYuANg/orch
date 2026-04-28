package placement

import (
	"context"
	"fmt"
	"strings"

	deployv1 "github.com/daiyuang/orch/internal/deploy/v1alpha1"
	"github.com/daiyuang/orch/internal/nodecapacity"
)

// Choose selects the best node id to run the workload using catalog snapshots.
// Scheduler refreshes snapshots periodically; callers should refresh local host before choose when needed.
//
// Strategy (v1): among feasible nodes (memory + projected CPU headroom vs workload.Resources),
// pick lowest CPUUsagePercent, tie-break larger MemoryAvailBytes.
func Choose(ctx context.Context, w deployv1.Workload, catalog *nodecapacity.Catalog, localNodeID string) (string, error) {
	_ = ctx
	if catalog == nil || catalog.Len() == 0 {
		id := strings.TrimSpace(localNodeID)
		if id == "" {
			id = "local"
		}
		return id, nil
	}

	candidates := catalog.NodeIDs()
	var pref []string
	if w.Scheduling != nil {
		pref = w.Scheduling.PreferredNodes
	}
	if len(pref) > 0 {
		want := prefSet(pref)
		filtered := candidates[:0]
		for _, id := range candidates {
			if want[strings.TrimSpace(id)] {
				filtered = append(filtered, id)
			}
		}
		if len(filtered) == 0 {
			return "", fmt.Errorf("placement: no catalog nodes match scheduling.preferredNodes")
		}
		candidates = filtered
	}

	var reqCPU int64
	var reqMem int64
	if w.Resources != nil {
		reqCPU = w.Resources.CPUMillis
		reqMem = w.Resources.MemoryBytes
	}

	var best string
	var bestSnap nodecapacity.Snapshot
	found := false

	for _, id := range candidates {
		snap, ok := catalog.Get(id)
		if !ok {
			continue
		}
		if !feasible(reqCPU, reqMem, snap) {
			continue
		}
		if !found || better(snap, bestSnap) {
			found = true
			best = id
			bestSnap = snap
		}
	}

	if !found {
		return "", fmt.Errorf("placement: no feasible node for workload %q (resources / stale catalog)", w.Name)
	}
	return best, nil
}

func prefSet(names []string) map[string]bool {
	m := make(map[string]bool, len(names))
	for _, n := range names {
		n = strings.TrimSpace(n)
		if n != "" {
			m[n] = true
		}
	}
	return m
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
	availFrac := (100.0 - p) / 100.0
	return int64(float64(total) * availFrac)
}

func better(a, b nodecapacity.Snapshot) bool {
	if a.CPUUsagePercent != b.CPUUsagePercent {
		return a.CPUUsagePercent < b.CPUUsagePercent
	}
	return a.MemoryAvailBytes > b.MemoryAvailBytes
}
