package api_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/lyonbrown4d/orch/internal/api"
	"github.com/lyonbrown4d/orch/internal/config"
	"github.com/lyonbrown4d/orch/internal/raftsvc"
)

type fakeRaftStatus struct {
	status raftsvc.Status
	err    error
}

func (f fakeRaftStatus) Status(context.Context) (raftsvc.Status, error) {
	return f.status, f.err
}

type leaderForwarderTestServer struct {
	t       *testing.T
	server  *httptest.Server
	gotPath string
}

func newLeaderForwarderTestServer(t *testing.T) *leaderForwarderTestServer {
	t.Helper()
	leader := &leaderForwarderTestServer{t: t}
	leader.server = httptest.NewServer(http.HandlerFunc(leader.handleDeploy))
	t.Cleanup(leader.server.Close)
	return leader
}

func (s *leaderForwarderTestServer) handleDeploy(w http.ResponseWriter, r *http.Request) {
	s.gotPath = r.URL.Path
	if !s.requirePostMethod(w, r) || !s.requireDeployBody(w, r) {
		return
	}
	writeForwardedDeployResponse(w)
}

func (s *leaderForwarderTestServer) requirePostMethod(w http.ResponseWriter, r *http.Request) bool {
	s.t.Helper()
	if r.Method != http.MethodPost {
		http.Error(w, "unexpected method", http.StatusMethodNotAllowed)
		s.t.Errorf("method = %s", r.Method)
		return false
	}
	return true
}

func (s *leaderForwarderTestServer) requireDeployBody(w http.ResponseWriter, r *http.Request) bool {
	s.t.Helper()
	var body map[string]string
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		s.t.Errorf("decode request: %v", err)
		return false
	}
	if body["name"] != "demo" {
		http.Error(w, "unexpected body", http.StatusBadRequest)
		s.t.Errorf("body = %#v", body)
		return false
	}
	return true
}

func writeForwardedDeployResponse(w http.ResponseWriter) {
	if err := json.NewEncoder(w).Encode(map[string]any{
		"accepted":  true,
		"app":       "demo",
		"workloads": 1,
	}); err != nil {
		http.Error(w, "encode response", http.StatusInternalServerError)
	}
}

func TestLeaderForwarderForwardsPostToConfiguredLeader(t *testing.T) {
	leader := newLeaderForwarderTestServer(t)
	cfg := config.Default()
	cfg.Cluster.Nodes = map[string]string{"node-a": leader.server.URL}
	forwarder := api.NewLeaderForwarder(
		cfg,
		fakeRaftStatus{status: raftsvc.Status{
			Ready:    true,
			IsLeader: false,
			LeaderID: "node-a",
		}},
	)

	var out struct {
		Accepted  bool   `json:"accepted"`
		App       string `json:"app"`
		Workloads int    `json:"workloads"`
	}
	forwarded, err := forwarder.ForwardPost(context.Background(), api.PathV1Deploy, map[string]string{"name": "demo"}, &out)
	if err != nil {
		t.Fatal(err)
	}
	if !forwarded {
		t.Fatal("expected forwarded request")
	}
	requireForwardedDeployResult(t, leader.gotPath, out.Accepted, out.App, out.Workloads)
}

func requireForwardedDeployResult(t *testing.T, gotPath string, accepted bool, app string, workloads int) {
	t.Helper()
	if gotPath != api.PathV1Deploy {
		t.Fatalf("path = %q, want %q", gotPath, api.PathV1Deploy)
	}
	if !accepted || app != "demo" || workloads != 1 {
		t.Fatalf("response = accepted:%t app:%q workloads:%d", accepted, app, workloads)
	}
}

func TestLeaderForwarderSkipsWhenLocalLeader(t *testing.T) {
	forwarder := api.NewLeaderForwarder(
		config.Default(),
		fakeRaftStatus{status: raftsvc.Status{
			Ready:    true,
			IsLeader: true,
			LeaderID: "node-a",
		}},
	)

	forwarded, err := forwarder.ForwardPost(context.Background(), api.PathV1Deploy, map[string]string{"name": "demo"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if forwarded {
		t.Fatal("local leader should not forward")
	}
}

func TestLeaderForwarderRequiresConfiguredLeaderAPI(t *testing.T) {
	forwarder := api.NewLeaderForwarder(
		config.Default(),
		fakeRaftStatus{status: raftsvc.Status{
			Ready:    true,
			IsLeader: false,
			LeaderID: "node-a",
		}},
	)

	forwarded, err := forwarder.ForwardPost(context.Background(), api.PathV1Deploy, map[string]string{"name": "demo"}, nil)
	if err == nil {
		t.Fatal("expected missing leader API error")
	}
	if forwarded {
		t.Fatal("request should not be forwarded without leader API URL")
	}
}
