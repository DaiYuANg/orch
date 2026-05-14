package raftsvc

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/arcgolabs/collectionx/list"

	"github.com/lyonbrown4d/orch/internal/nodecapacity"
	"github.com/lyonbrown4d/orch/pkg/oopsx"
)

// NewRaftCapacityStore returns a [nodecapacity.SnapshotStore] backed by the replicated FSM (raft apply).
func NewRaftCapacityStore(s *Service) nodecapacity.SnapshotStore {
	return &raftCapacityStore{s: s}
}

type raftCapacityStore struct {
	s *Service
}

func (r *raftCapacityStore) Upsert(ctx context.Context, snap nodecapacity.Snapshot) error {
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
		return oopsx.B("raft", "capacity").Wrapf(err, "encode node capacity")
	}
	if r.s.nh == nil {
		r.s.fsm.applyCommandPayload(b)
		return nil
	}
	if !r.s.isLocalLeader() {
		return nil
	}
	if err := r.s.applyCommand(ctx, b, 5*time.Second, "not leader: send node capacity to the raft leader node"); err != nil {
		return oopsx.B("raft", "capacity").Wrapf(err, "apply node capacity")
	}
	return nil
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
