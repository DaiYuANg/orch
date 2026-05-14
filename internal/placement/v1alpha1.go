package placement

import (
	"context"
	"errors"
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
		return fallbackNodeID(localNodeID), nil
	}

	candidates, preferences, restricted, err := placementCandidates(w, catalog)
	if err != nil {
		return "", err
	}
	best := bestPlacementCandidate(candidates, catalog, workloadRequest(w), preferences, restricted)
	node, ok := best.Get()
	if !ok {
		return "", fmt.Errorf("placement: no feasible node for workload %q (resources / stale catalog)", w.Name)
	}
	return node.id, nil
}

type scoredNode struct {
	id   string
	snap nodecapacity.Snapshot
}

type workloadResourceRequest struct {
	cpuMillis  int64
	memoryByte int64
}

func fallbackNodeID(localNodeID string) string {
	id := strings.TrimSpace(localNodeID)
	if id == "" {
		return "local"
	}
	return id
}

func placementCandidates(
	w deployv1.Workload,
	catalog *nodecapacity.Catalog,
) (*list.List[string], *set.OrderedSet[string], bool, error) {
	candidates := catalog.NodeIDs()
	want, restricted := preferredNames(w)
	if !restricted {
		return candidates, want, false, nil
	}
	if want.IsEmpty() {
		return nil, nil, true, errors.New("placement: scheduling.preferredNodes has no valid names")
	}
	filtered := list.FilterList(candidates, func(_ int, id string) bool {
		return want.Contains(strings.TrimSpace(id))
	})
	if filtered.Len() == 0 {
		return nil, nil, true, errors.New("placement: no catalog nodes match scheduling.preferredNodes")
	}
	return filtered, want, true, nil
}

func workloadRequest(w deployv1.Workload) workloadResourceRequest {
	if w.Resources == nil {
		return workloadResourceRequest{}
	}
	return workloadResourceRequest{
		cpuMillis:  w.Resources.CPUMillis,
		memoryByte: w.Resources.MemoryBytes,
	}
}

func bestPlacementCandidate(
	candidates *list.List[string],
	catalog *nodecapacity.Catalog,
	request workloadResourceRequest,
	preferences *set.OrderedSet[string],
	restricted bool,
) mo.Option[scoredNode] {
	var best mo.Option[scoredNode]
	candidates.Range(func(_ int, id string) bool {
		best = chooseBetterCandidate(best, catalog, request, preferences, restricted, id)
		return true
	})
	return best
}

func chooseBetterCandidate(
	best mo.Option[scoredNode],
	catalog *nodecapacity.Catalog,
	request workloadResourceRequest,
	preferences *set.OrderedSet[string],
	restricted bool,
	id string,
) mo.Option[scoredNode] {
	snap, ok := catalog.Get(id)
	if !ok || !feasible(request.cpuMillis, request.memoryByte, snap) {
		return best
	}
	cur := scoredNode{id: id, snap: snap}
	prev, ok := best.Get()
	if !ok || shouldReplaceCandidate(cur, prev, preferences, restricted) {
		return mo.Some(cur)
	}
	return best
}

func shouldReplaceCandidate(cur, prev scoredNode, preferences *set.OrderedSet[string], restricted bool) bool {
	if better(cur.snap, prev.snap) {
		return true
	}
	return restricted &&
		snapEqual(cur.snap, prev.snap) &&
		preferredRank(preferences, cur.id) < preferredRank(preferences, prev.id)
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
	found := want.Len() + 1
	index := 0
	want.Range(func(id string) bool {
		if id == nodeID {
			found = index
			return false
		}
		index++
		return true
	})
	return found
}

func feasible(reqCPUmillis, reqMemBytes int64, s nodecapacity.Snapshot) bool {
	if reqMemBytes > 0 && s.MemoryAvailBytes < uint64(reqMemBytes) {
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
