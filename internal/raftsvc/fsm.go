package raftsvc

import (
	"encoding/json"
	"io"
	"strings"
	"sync"

	"github.com/arcgolabs/collectionx/list"
	sm "github.com/lni/dragonboat/v4/statemachine"

	deployv1 "github.com/daiyuang/orch/internal/deploy/v1alpha1"
	"github.com/daiyuang/orch/internal/nodecapacity"
	"github.com/daiyuang/orch/internal/workloadmeta"
	"github.com/daiyuang/orch/pkg/oopsx"
)

const (
	cmdUpsertNodeCapacity       = "upsert_node_capacity"
	cmdUpsertDeployApp          = "upsert_deploy_app"
	cmdDeleteDeployApp          = "delete_deploy_app"
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

func (f *schedulingFSM) Update(entry sm.Entry) (sm.Result, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.state.AppliedCommands++
	if len(entry.Cmd) == 0 {
		return sm.Result{Value: f.state.AppliedCommands}, nil
	}
	f.applyPayloadLocked(entry.Cmd)
	return sm.Result{Value: f.state.AppliedCommands}, nil
}

func (f *schedulingFSM) Lookup(interface{}) (interface{}, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.state, nil
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
	case cmdDeleteDeployApp:
		var env struct {
			Type     string            `json:"type"`
			Metadata deployv1.Metadata `json:"metadata"`
		}
		if err := json.Unmarshal(data, &env); err != nil {
			return
		}
		if strings.TrimSpace(env.Metadata.Name) == "" {
			return
		}
		if f.state.DeployApps != nil {
			delete(f.state.DeployApps, deployAppMapKey(env.Metadata))
		}
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
	return workloadmeta.NamespaceOrDefault(m.Namespace) + "/" + strings.TrimSpace(m.Name)
}

func (f *schedulingFSM) getDeployApp(meta deployv1.Metadata) (deployv1.App, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.state.DeployApps == nil {
		return deployv1.App{}, false
	}
	app, ok := f.state.DeployApps[deployAppMapKey(meta)]
	return app, ok
}

func (f *schedulingFSM) listDeployApps() *list.List[deployv1.App] {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.state.DeployApps) == 0 {
		return list.NewList[deployv1.App]()
	}
	out := list.NewListWithCapacity[deployv1.App](len(f.state.DeployApps))
	for _, app := range f.state.DeployApps {
		out.Add(app)
	}
	return out
}

func (f *schedulingFSM) listAssignments() *list.List[workloadmeta.Assignment] {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.state.Assignments) == 0 {
		return list.NewList[workloadmeta.Assignment]()
	}
	out := list.NewListWithCapacity[workloadmeta.Assignment](len(f.state.Assignments))
	for _, a := range f.state.Assignments {
		out.Add(a)
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

func (f *schedulingFSM) SaveSnapshot(w io.Writer, _ sm.ISnapshotFileCollection, _ <-chan struct{}) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	b, err := json.Marshal(f.state)
	if err != nil {
		return oopsx.B("raft").Wrapf(err, "snapshot marshal")
	}
	if _, err := w.Write(b); err != nil {
		return oopsx.B("raft").Wrapf(err, "snapshot write")
	}
	return nil
}

func (f *schedulingFSM) RecoverFromSnapshot(r io.Reader, _ []sm.SnapshotFile, _ <-chan struct{}) error {
	data, err := io.ReadAll(r)
	if err != nil {
		return oopsx.B("raft").Wrapf(err, "fsm restore read snapshot")
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

func (f *schedulingFSM) Close() error {
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
