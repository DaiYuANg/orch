package raftsvc

import (
	"encoding/json"
	"io"
	"strings"
	"sync"

	hraft "github.com/hashicorp/raft"

	deployv1 "github.com/daiyuang/orch/internal/deploy/v1alpha1"
	"github.com/daiyuang/orch/internal/nodecapacity"
	"github.com/daiyuang/orch/internal/workloadmeta"
	"github.com/daiyuang/orch/pkg/oopsx"
)

const (
	cmdUpsertNodeCapacity       = "upsert_node_capacity"
	cmdUpsertDeployApp          = "upsert_deploy_app"
	cmdUpsertWorkloadAssignment = "upsert_workload_assignment"
)

// schedulingFSM holds replicated control-plane state (node capacity snapshots, etc.).
type schedulingFSM struct {
	mu           sync.Mutex
	state        fsmSnapshotState
	notifyDeploy func()
}

type fsmSnapshotState struct {
	AppliedCommands uint64                             `json:"appliedCommands"`
	NodeCapacity    map[string]nodecapacity.Snapshot   `json:"nodeCapacity,omitempty"`
	DeployApps      map[string]deployv1.App            `json:"deployApps,omitempty"`
	Assignments     map[string]workloadmeta.Assignment `json:"assignments,omitempty"`
}

func (f *schedulingFSM) setNotifyDeploy(fn func()) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.notifyDeploy = fn
}

func (f *schedulingFSM) Apply(l *hraft.Log) any {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.state.AppliedCommands++
	if len(l.Data) == 0 {
		return f.state.AppliedCommands
	}
	f.applyPayloadLocked(l.Data)
	return f.state.AppliedCommands
}

// applyCommandPayload applies a replicated (or local single-node) command without going through the Raft log reader.
func (f *schedulingFSM) applyCommandPayload(data []byte) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.state.AppliedCommands++
	if len(data) == 0 {
		return
	}
	f.applyPayloadLocked(data)
}

func (f *schedulingFSM) applyPayloadLocked(data []byte) {
	var head struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(data, &head); err != nil {
		return
	}
	switch head.Type {
	case cmdUpsertNodeCapacity:
		var env struct {
			Type string                `json:"type"`
			Node nodecapacity.Snapshot `json:"node"`
		}
		if err := json.Unmarshal(data, &env); err != nil {
			return
		}
		id := strings.TrimSpace(env.Node.NodeID)
		if id == "" {
			return
		}
		if f.state.NodeCapacity == nil {
			f.state.NodeCapacity = make(map[string]nodecapacity.Snapshot)
		}
		f.state.NodeCapacity[id] = env.Node
	case cmdUpsertDeployApp:
		var env struct {
			Type string       `json:"type"`
			App  deployv1.App `json:"app"`
		}
		if err := json.Unmarshal(data, &env); err != nil {
			return
		}
		if strings.TrimSpace(env.App.Metadata.Name) == "" {
			return
		}
		key := deployAppMapKey(env.App.Metadata)
		if f.state.DeployApps == nil {
			f.state.DeployApps = make(map[string]deployv1.App)
		}
		f.state.DeployApps[key] = env.App
		if f.notifyDeploy != nil {
			f.notifyDeploy()
		}
	case cmdUpsertWorkloadAssignment:
		var env struct {
			Type       string                  `json:"type"`
			Assignment workloadmeta.Assignment `json:"assignment"`
		}
		if err := json.Unmarshal(data, &env); err != nil {
			return
		}
		assignment := env.Assignment
		assignment.Key = strings.TrimSpace(assignment.Key)
		assignment.Metadata.Name = strings.TrimSpace(assignment.Metadata.Name)
		assignment.Metadata.Namespace = strings.TrimSpace(assignment.Metadata.Namespace)
		assignment.Workload = strings.TrimSpace(assignment.Workload)
		assignment.Node = strings.TrimSpace(assignment.Node)
		assignment.Status = strings.TrimSpace(assignment.Status)
		if assignment.Metadata.Name == "" || assignment.Workload == "" {
			return
		}
		if assignment.Key == "" {
			assignment.Key = workloadmeta.AssignmentKey(assignment.Metadata, assignment.Workload)
		}
		if assignment.Key == "" {
			return
		}
		if f.state.Assignments == nil {
			f.state.Assignments = make(map[string]workloadmeta.Assignment)
		}
		f.state.Assignments[assignment.Key] = assignment
	default:
		return
	}
}

func deployAppMapKey(m deployv1.Metadata) string {
	return strings.TrimSpace(m.Namespace) + "/" + strings.TrimSpace(m.Name)
}

func (f *schedulingFSM) listDeployApps() []deployv1.App {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.state.DeployApps) == 0 {
		return nil
	}
	out := make([]deployv1.App, 0, len(f.state.DeployApps))
	for _, app := range f.state.DeployApps {
		out = append(out, app)
	}
	return out
}

func (f *schedulingFSM) listAssignments() []workloadmeta.Assignment {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.state.Assignments) == 0 {
		return nil
	}
	out := make([]workloadmeta.Assignment, 0, len(f.state.Assignments))
	for _, a := range f.state.Assignments {
		out = append(out, a)
	}
	return out
}

func (f *schedulingFSM) getAssignment(key string) (workloadmeta.Assignment, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.state.Assignments == nil {
		return workloadmeta.Assignment{}, false
	}
	a, ok := f.state.Assignments[strings.TrimSpace(key)]
	return a, ok
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
