package raftsvc

import (
	"bytes"
	"encoding/json"
	"io"
	"testing"

	hraft "github.com/hashicorp/raft"

	deployv1 "github.com/daiyuang/orch/internal/deploy/v1alpha1"
)

func TestFSMApplyDeployApp(t *testing.T) {
	f := &schedulingFSM{}
	app := deployv1.App{}
	app.Metadata.Name = "demo"
	app.Metadata.Namespace = "ns1"
	app.Workloads = []deployv1.Workload{{Name: "w", Runtime: deployv1.RuntimeDocker}}

	b, err := json.Marshal(struct {
		Type string       `json:"type"`
		App  deployv1.App `json:"app"`
	}{Type: cmdUpsertDeployApp, App: app})
	if err != nil {
		t.Fatal(err)
	}
	f.applyCommandPayload(b)
	apps := f.listDeployApps()
	if len(apps) != 1 || apps[0].Metadata.Name != "demo" {
		t.Fatalf("list = %#v", apps)
	}
}

func TestFSMDeploySnapshotRoundTrip(t *testing.T) {
	f := &schedulingFSM{}
	app := deployv1.App{}
	app.Metadata.Name = "demo"
	b, err := json.Marshal(struct {
		Type string       `json:"type"`
		App  deployv1.App `json:"app"`
	}{Type: cmdUpsertDeployApp, App: app})
	if err != nil {
		t.Fatal(err)
	}
	f.applyCommandPayload(b)

	snap, err := f.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	sink := &memSnapSink{id: "t1"}
	if err := snap.Persist(sink); err != nil {
		t.Fatal(err)
	}
	snap.Release()

	f2 := &schedulingFSM{}
	if err := f2.Restore(io.NopCloser(bytes.NewReader(sink.Bytes()))); err != nil {
		t.Fatal(err)
	}
	apps := f2.listDeployApps()
	if len(apps) != 1 || apps[0].Metadata.Name != "demo" {
		t.Fatalf("after restore = %#v", apps)
	}
}

type memSnapSink struct {
	id string
	bytes.Buffer
}

func (m *memSnapSink) ID() string    { return m.id }
func (m *memSnapSink) Cancel() error { return nil }
func (m *memSnapSink) Close() error  { return nil }

var _ hraft.SnapshotSink = (*memSnapSink)(nil)
