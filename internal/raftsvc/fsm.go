package raftsvc

import (
	"encoding/json"
	"io"
	"sync"

	hraft "github.com/hashicorp/raft"
)

// schedulingFSM is a minimal replicated state machine placeholder; scheduling
// payloads can later decode Log.Data here.
type schedulingFSM struct {
	mu    sync.Mutex
	state fsmSnapshotState
}

type fsmSnapshotState struct {
	AppliedCommands uint64 `json:"appliedCommands"`
}

func (f *schedulingFSM) Apply(l *hraft.Log) any {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.state.AppliedCommands++
	return f.state.AppliedCommands
}

func (f *schedulingFSM) Snapshot() (hraft.FSMSnapshot, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	st := f.state
	return &schedulingSnapshot{payload: st}, nil
}

func (f *schedulingFSM) Restore(rc io.ReadCloser) error {
	defer rc.Close()
	data, err := io.ReadAll(rc)
	if err != nil {
		return err
	}
	var st fsmSnapshotState
	if len(data) > 0 {
		if err := json.Unmarshal(data, &st); err != nil {
			return err
		}
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.state = st
	return nil
}

type schedulingSnapshot struct {
	payload fsmSnapshotState
}

func (s *schedulingSnapshot) Persist(sink hraft.SnapshotSink) error {
	b, err := json.Marshal(s.payload)
	if err != nil {
		_ = sink.Cancel()
		return err
	}
	if _, err := sink.Write(b); err != nil {
		_ = sink.Cancel()
		return err
	}
	return sink.Close()
}

func (s *schedulingSnapshot) Release() {}
