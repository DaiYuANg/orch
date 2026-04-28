package raftsvc

import (
	"encoding/json"
	"io"
	"strings"
	"sync"

	hraft "github.com/hashicorp/raft"

	"github.com/daiyuang/orch/internal/nodecapacity"
	"github.com/daiyuang/orch/pkg/oopsx"
)

const cmdUpsertNodeCapacity = "upsert_node_capacity"

// schedulingFSM holds replicated control-plane state (node capacity snapshots, etc.).
type schedulingFSM struct {
	mu    sync.Mutex
	state fsmSnapshotState
}

type fsmSnapshotState struct {
	AppliedCommands uint64                           `json:"appliedCommands"`
	NodeCapacity    map[string]nodecapacity.Snapshot `json:"nodeCapacity,omitempty"`
}

func (f *schedulingFSM) Apply(l *hraft.Log) any {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.state.AppliedCommands++
	if len(l.Data) == 0 {
		return f.state.AppliedCommands
	}
	var env struct {
		Type string                `json:"type"`
		Node nodecapacity.Snapshot `json:"node"`
	}
	if err := json.Unmarshal(l.Data, &env); err != nil {
		return f.state.AppliedCommands
	}
	if env.Type != cmdUpsertNodeCapacity {
		return f.state.AppliedCommands
	}
	id := strings.TrimSpace(env.Node.NodeID)
	if id == "" {
		return f.state.AppliedCommands
	}
	if f.state.NodeCapacity == nil {
		f.state.NodeCapacity = make(map[string]nodecapacity.Snapshot)
	}
	f.state.NodeCapacity[id] = env.Node
	return f.state.AppliedCommands
}

func (f *schedulingFSM) Snapshot() (hraft.FSMSnapshot, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	st := f.state
	return &schedulingSnapshot{payload: st}, nil
}

func (f *schedulingFSM) Restore(rc io.ReadCloser) error {
	data, err := io.ReadAll(rc)
	closeErr := rc.Close()
	if err != nil {
		return oopsx.B("raft").Wrapf(err, "fsm restore read snapshot")
	}
	if closeErr != nil {
		return oopsx.B("raft").Wrapf(closeErr, "fsm restore close reader")
	}
	var st fsmSnapshotState
	if len(data) > 0 {
		if err := json.Unmarshal(data, &st); err != nil {
			return oopsx.B("raft").Wrapf(err, "fsm restore unmarshal")
		}
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.state = st
	return nil
}

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

func (f *schedulingFSM) nodeCapacityIDs() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.state.NodeCapacity == nil {
		return nil
	}
	out := make([]string, 0, len(f.state.NodeCapacity))
	for id := range f.state.NodeCapacity {
		out = append(out, id)
	}
	return out
}

type schedulingSnapshot struct {
	payload fsmSnapshotState
}

func (s *schedulingSnapshot) Persist(sink hraft.SnapshotSink) error {
	b, err := json.Marshal(s.payload)
	if err != nil {
		cancelErr := sink.Cancel()
		if cancelErr != nil {
			return oopsx.B("raft").Wrapf(err, "snapshot marshal (cancel: %v)", cancelErr)
		}
		return oopsx.B("raft").Wrapf(err, "snapshot marshal")
	}
	if _, err := sink.Write(b); err != nil {
		cancelErr := sink.Cancel()
		if cancelErr != nil {
			return oopsx.B("raft").Wrapf(err, "snapshot write (cancel: %v)", cancelErr)
		}
		return oopsx.B("raft").Wrapf(err, "snapshot write")
	}
	if err := sink.Close(); err != nil {
		return oopsx.B("raft").Wrapf(err, "snapshot close sink")
	}
	return nil
}

func (s *schedulingSnapshot) Release() {}
