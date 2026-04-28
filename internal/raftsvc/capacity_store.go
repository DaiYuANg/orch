package raftsvc

import (
	"context"
	"encoding/json"
	"sort"
	"strings"
	"time"

	hraft "github.com/hashicorp/raft"

	"github.com/daiyuang/orch/internal/nodecapacity"
)

// NewRaftCapacityStore returns a [nodecapacity.SnapshotStore] backed by the replicated FSM (raft apply).
func NewRaftCapacityStore(s *Service) nodecapacity.SnapshotStore {
	return &raftCapacityStore{s: s}
}

type raftCapacityStore struct {
	s *Service
}

func (r *raftCapacityStore) Upsert(ctx context.Context, snap nodecapacity.Snapshot) error {
	_ = ctx
	if r == nil || r.s == nil {
		return nil
	}
	if r.s.r == nil {
		return nil
	}
	if r.s.r.State() != hraft.Leader {
		return nil
	}
	env := struct {
		Type string                `json:"type"`
		Node nodecapacity.Snapshot `json:"node"`
	}{
		Type: cmdUpsertNodeCapacity,
		Node: snap,
	}
	b, err := json.Marshal(env)
	if err != nil {
		return err
	}
	f := r.s.r.Apply(b, 5*time.Second)
	return f.Error()
}

func (r *raftCapacityStore) Get(nodeID string) (nodecapacity.Snapshot, bool) {
	if r == nil || r.s == nil || r.s.fsm == nil {
		return nodecapacity.Snapshot{}, false
	}
	return r.s.fsm.getNodeCapacity(strings.TrimSpace(nodeID))
}

func (r *raftCapacityStore) Len() int {
	if r == nil || r.s == nil || r.s.fsm == nil {
		return 0
	}
	return r.s.fsm.lenNodeCapacity()
}

func (r *raftCapacityStore) NodeIDs() []string {
	if r == nil || r.s == nil || r.s.fsm == nil {
		return nil
	}
	ids := r.s.fsm.nodeCapacityIDs()
	sort.Strings(ids)
	return ids
}
