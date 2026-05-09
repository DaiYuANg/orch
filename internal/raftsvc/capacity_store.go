package raftsvc

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/arcgolabs/collectionx/list"

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
	if !r.s.cfg.Raft.Enabled {
		r.s.fsm.applyCommandPayload(b)
		return nil
	}
	if r.s.nh == nil {
		return nil
	}
	if !r.s.isLocalLeader() {
		return nil
	}
	return r.s.applyCommand(b, 5*time.Second, "not leader: send node capacity to the raft leader node")
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

func (r *raftCapacityStore) NodeIDs() *list.List[string] {
	if r == nil || r.s == nil || r.s.fsm == nil {
		return list.NewList[string]()
	}
	ids := r.s.fsm.nodeCapacityIDs()
	ids.Sort(strings.Compare)
	return ids
}
