package raftsvc

import (
	"context"
	"io"
	"log/slog"
	"path/filepath"
	"strings"
	"testing"
	"time"

	hraft "github.com/hashicorp/raft"

	"github.com/daiyuang/orch/internal/config"
	"github.com/daiyuang/orch/internal/nodeid"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestBootstrapServerListIncludesStaticPeers(t *testing.T) {
	cfg := config.Default()
	cfg.Raft.Peers = map[string]string{
		"node-b": "10.0.0.12:7444",
		"node-a": "10.0.0.11:7444",
	}
	svc := New(cfg, testLogger(), nodeid.Local{Value: "node-a"})

	servers, err := svc.bootstrapServerList(hraft.ServerID("node-a"), hraft.ServerAddress("10.0.0.11:7444"))
	if err != nil {
		t.Fatal(err)
	}

	got := servers.Values()
	if len(got) != 2 {
		t.Fatalf("servers len = %d, want 2: %#v", len(got), got)
	}
	if got[0].ID != "node-a" || got[0].Address != "10.0.0.11:7444" {
		t.Fatalf("first server = %#v", got[0])
	}
	if got[1].ID != "node-b" || got[1].Address != "10.0.0.12:7444" {
		t.Fatalf("second server = %#v", got[1])
	}
}

func TestBootstrapServerListOverridesLocalPeerAddress(t *testing.T) {
	cfg := config.Default()
	cfg.Raft.Peers = map[string]string{
		"node-a": "10.0.0.99:7444",
	}
	svc := New(cfg, testLogger(), nodeid.Local{Value: "node-a"})

	servers, err := svc.bootstrapServerList(hraft.ServerID("node-a"), hraft.ServerAddress("10.0.0.11:7444"))
	if err != nil {
		t.Fatal(err)
	}

	got := servers.Values()
	if len(got) != 1 {
		t.Fatalf("servers len = %d, want 1: %#v", len(got), got)
	}
	if got[0].Address != "10.0.0.11:7444" {
		t.Fatalf("local address = %q, want transport address", got[0].Address)
	}
}

func TestBootstrapServerListRejectsPeerWithoutHost(t *testing.T) {
	cfg := config.Default()
	cfg.Raft.Peers = map[string]string{"node-b": ":7444"}
	svc := New(cfg, testLogger(), nodeid.Local{Value: "node-a"})

	if _, err := svc.bootstrapServerList(hraft.ServerID("node-a"), hraft.ServerAddress("10.0.0.11:7444")); err == nil {
		t.Fatal("expected invalid peer error")
	}
}

func TestRaftMembershipSingleNode(t *testing.T) {
	ctx := context.Background()
	svc := newStartedTestRaft(t, "node-a", true)
	waitRaftState(t, svc, hraft.Leader)

	members, err := svc.ListMembers(ctx)
	if err != nil {
		t.Fatal(err)
	}
	got := members.Values()
	if len(got) != 1 {
		t.Fatalf("members len = %d, want 1: %#v", len(got), got)
	}
	if got[0].ID != "node-a" || got[0].Address == "" || got[0].Suffrage != hraft.Voter.String() {
		t.Fatalf("member = %#v", got[0])
	}
}

func TestRaftStatusSingleNodeLeader(t *testing.T) {
	ctx := context.Background()
	svc := newStartedTestRaft(t, "node-a", true)
	waitRaftState(t, svc, hraft.Leader)

	status, err := svc.Status(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !status.Enabled || !status.Ready || !status.IsLeader {
		t.Fatalf("status readiness = enabled:%t ready:%t leader:%t", status.Enabled, status.Ready, status.IsLeader)
	}
	if status.NodeID != "node-a" || status.LeaderID != "node-a" {
		t.Fatalf("status ids = node:%q leader:%q", status.NodeID, status.LeaderID)
	}
	if status.State != hraft.Leader.String() || status.LocalAddress == "" || status.LeaderAddress == "" {
		t.Fatalf("status = %#v", status)
	}
	if status.Members == nil || status.Members.Len() != 1 {
		t.Fatalf("status members = %#v", status.Members)
	}
}

func TestRaftAddAndRemoveVoter(t *testing.T) {
	ctx := context.Background()
	leader := newStartedTestRaft(t, "node-a", true)
	waitRaftState(t, leader, hraft.Leader)

	follower := newStartedTestRaft(t, "node-b", false)
	followerAddr := string(follower.transport.LocalAddr())
	if strings.TrimSpace(followerAddr) == "" {
		t.Fatal("follower raft address is empty")
	}

	if err := leader.AddVoter(ctx, "node-b", followerAddr); err != nil {
		t.Fatal(err)
	}
	waitRaftMember(t, leader, "node-b", true)

	if err := leader.RemoveServer(ctx, "node-b"); err != nil {
		t.Fatal(err)
	}
	waitRaftMember(t, leader, "node-b", false)
}

func newStartedTestRaft(t *testing.T, id string, bootstrap bool) *Service {
	t.Helper()
	tmp := t.TempDir()
	cfg := config.Default()
	cfg.Raft.Bind = "127.0.0.1:0"
	cfg.Raft.Advertise = ""
	cfg.Raft.Bootstrap = bootstrap
	cfg.Raft.Peers = map[string]string{}
	cfg.Raft.Badger.Dir = filepath.Join(tmp, "badger")
	cfg.Raft.Bolt.Path = filepath.Join(tmp, "raft-meta.db")
	cfg.Raft.Snapshot.Dir = filepath.Join(tmp, "snapshots")

	svc := New(cfg, testLogger(), nodeid.Local{Value: id})
	if err := svc.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		stopCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := svc.Stop(stopCtx); err != nil {
			t.Logf("stop raft: %v", err)
		}
	})
	return svc
}

func waitRaftState(t *testing.T, svc *Service, want hraft.RaftState) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		if svc.r != nil && svc.r.State() == want {
			return
		}
		if time.Now().After(deadline) {
			if svc.r == nil {
				t.Fatalf("raft did not reach %s: raft nil", want)
			}
			t.Fatalf("raft state = %s, want %s", svc.r.State(), want)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func waitRaftMember(t *testing.T, svc *Service, id string, want bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		members, err := svc.ListMembers(context.Background())
		if err == nil {
			found := false
			members.Range(func(_ int, member Member) bool {
				if member.ID == id {
					found = true
					return false
				}
				return true
			})
			if found == want {
				return
			}
		}
		if time.Now().After(deadline) {
			t.Fatalf("raft member %q present=%t, want %t", id, !want, want)
		}
		time.Sleep(20 * time.Millisecond)
	}
}
