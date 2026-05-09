package raftsvc

import (
	"context"
	"io"
	"log/slog"
	"net"
	"path/filepath"
	"testing"
	"time"

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

	servers, err := svc.bootstrapServerList("node-a", "10.0.0.11:7444")
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

	servers, err := svc.bootstrapServerList("node-a", "10.0.0.11:7444")
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

	if _, err := svc.bootstrapServerList("node-a", "10.0.0.11:7444"); err == nil {
		t.Fatal("expected invalid peer error")
	}
}

func TestRaftMembershipSingleNode(t *testing.T) {
	ctx := context.Background()
	svc := newStartedTestRaft(t, "node-a", true)
	waitRaftLeader(t, svc)

	members, err := svc.ListMembers(ctx)
	if err != nil {
		t.Fatal(err)
	}
	got := members.Values()
	if len(got) != 1 {
		t.Fatalf("members len = %d, want 1: %#v", len(got), got)
	}
	if got[0].ID != "node-a" || got[0].Address == "" || got[0].Suffrage != "Voter" {
		t.Fatalf("member = %#v", got[0])
	}
}

func TestRaftStatusSingleNodeLeader(t *testing.T) {
	ctx := context.Background()
	svc := newStartedTestRaft(t, "node-a", true)
	waitRaftLeader(t, svc)

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
	if status.State != "Leader" || status.LocalAddress == "" || status.LeaderAddress == "" {
		t.Fatalf("status = %#v", status)
	}
	if status.Members == nil || status.Members.Len() != 1 {
		t.Fatalf("status members = %#v", status.Members)
	}
}

func TestRaftAddAndRemoveVoter(t *testing.T) {
	ctx := context.Background()
	leader := newStartedTestRaft(t, "node-a", true)
	waitRaftLeader(t, leader)

	followerAddr := reserveTestRaftAddr(t)

	if err := leader.AddVoter(ctx, "node-b", followerAddr); err != nil {
		t.Fatal(err)
	}
	follower := newStartedTestRaftWithBind(t, "node-b", false, followerAddr)
	_ = follower
	waitRaftMember(t, leader, "node-b", true)

	if err := leader.RemoveServer(ctx, "node-b"); err != nil {
		t.Fatal(err)
	}
	waitRaftMember(t, leader, "node-b", false)
}

func newStartedTestRaft(t testing.TB, id string, bootstrap bool) *Service {
	return newStartedTestRaftWithBind(t, id, bootstrap, "127.0.0.1:0")
}

func newStartedTestRaftWithBind(t testing.TB, id string, bootstrap bool, bind string) *Service {
	t.Helper()
	tmp := t.TempDir()
	cfg := config.Default()
	cfg.Raft.Bind = bind
	cfg.Raft.Advertise = ""
	cfg.Raft.Bootstrap = bootstrap
	cfg.Raft.Peers = map[string]string{}
	cfg.Raft.Data.Dir = filepath.Join(tmp, "dragonboat")

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

func reserveTestRaftAddr(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	if err := ln.Close(); err != nil {
		t.Fatal(err)
	}
	return addr
}

func waitRaftLeader(t testing.TB, svc *Service) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for {
		if svc.isLocalLeader() {
			return
		}
		if time.Now().After(deadline) {
			status, _ := svc.Status(context.Background())
			t.Fatalf("raft did not reach leader: %#v", status)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func waitRaftMember(t testing.TB, svc *Service, id string, want bool) {
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
