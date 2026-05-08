package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/daiyuang/orch/internal/config"
	"github.com/daiyuang/orch/internal/raftsvc"
)

type fakeRaftStatus struct {
	status raftsvc.Status
	err    error
}

func (f fakeRaftStatus) Status(context.Context) (raftsvc.Status, error) {
	return f.status, f.err
}

func TestLeaderForwarderForwardsPostToConfiguredLeader(t *testing.T) {
	var gotPath string
	leader := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s", r.Method)
		}
		var body map[string]string
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if body["name"] != "demo" {
			t.Fatalf("body = %#v", body)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"accepted":  true,
			"app":       "demo",
			"workloads": 1,
		})
	}))
	t.Cleanup(leader.Close)

	cfg := config.Default()
	cfg.Cluster.Nodes = map[string]string{"node-a": leader.URL}
	forwarder := &LeaderForwarder{
		cfg: cfg,
		raft: fakeRaftStatus{status: raftsvc.Status{
			Enabled:  true,
			Ready:    true,
			IsLeader: false,
			LeaderID: "node-a",
		}},
		client: leader.Client(),
	}

	var out struct {
		Accepted  bool   `json:"accepted"`
		App       string `json:"app"`
		Workloads int    `json:"workloads"`
	}
	forwarded, err := forwarder.ForwardPost(context.Background(), PathV1Deploy, map[string]string{"name": "demo"}, &out)
	if err != nil {
		t.Fatal(err)
	}
	if !forwarded {
		t.Fatal("expected forwarded request")
	}
	if gotPath != PathV1Deploy {
		t.Fatalf("path = %q, want %q", gotPath, PathV1Deploy)
	}
	if !out.Accepted || out.App != "demo" || out.Workloads != 1 {
		t.Fatalf("response = %#v", out)
	}
}

func TestLeaderForwarderSkipsWhenLocalLeader(t *testing.T) {
	forwarder := &LeaderForwarder{
		cfg: config.Default(),
		raft: fakeRaftStatus{status: raftsvc.Status{
			Enabled:  true,
			Ready:    true,
			IsLeader: true,
			LeaderID: "node-a",
		}},
	}

	forwarded, err := forwarder.ForwardPost(context.Background(), PathV1Deploy, map[string]string{"name": "demo"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if forwarded {
		t.Fatal("local leader should not forward")
	}
}

func TestLeaderForwarderRequiresConfiguredLeaderAPI(t *testing.T) {
	forwarder := &LeaderForwarder{
		cfg: config.Default(),
		raft: fakeRaftStatus{status: raftsvc.Status{
			Enabled:  true,
			Ready:    true,
			IsLeader: false,
			LeaderID: "node-a",
		}},
	}

	forwarded, err := forwarder.ForwardPost(context.Background(), PathV1Deploy, map[string]string{"name": "demo"}, nil)
	if err == nil {
		t.Fatal("expected missing leader API error")
	}
	if forwarded {
		t.Fatal("request should not be forwarded without leader API URL")
	}
}
