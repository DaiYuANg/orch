package raftsvc

import (
	"encoding/json"
	"io"
	"sync"

	hraft "github.com/hashicorp/raft"

	"github.com/daiyuang/orch/pkg/oopsx"
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
