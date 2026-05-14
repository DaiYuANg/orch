package raftsvc

import (
	"github.com/arcgolabs/collectionx/list"

	"github.com/daiyuang/orch/internal/nodecapacity"
)

func (f *schedulingFSM) getNodeCapacity(nodeID string) (nodecapacity.Snapshot, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.state.NodeCapacity == nil {
		return nodecapacity.Snapshot{}, false
	}
	s, ok := f.state.NodeCapacity[nodeID]
	return s, ok
}

func (f *schedulingFSM) lenNodeCapacity() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.state.NodeCapacity == nil {
		return 0
	}
	return len(f.state.NodeCapacity)
}

func (f *schedulingFSM) nodeCapacityIDs() *list.List[string] {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.state.NodeCapacity == nil {
		return list.NewList[string]()
	}
	out := list.NewListWithCapacity[string](len(f.state.NodeCapacity))
	for id := range f.state.NodeCapacity {
		out.Add(id)
	}
	return out
}
