package raftsvc_test

import (
	"context"
	"fmt"
	"maps"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/lyonbrown4d/orch/internal/config"
	deployv1 "github.com/lyonbrown4d/orch/internal/deploy/v1alpha1"
	"github.com/lyonbrown4d/orch/internal/nodeid"
	"github.com/lyonbrown4d/orch/internal/raftsvc"
)

func TestRaftThreeNodeLeaderFailoverReplicatesState(t *testing.T) {
	ctx := context.Background()
	nodeIDs := []string{"node-a", "node-b", "node-c"}
	peers := reserveTestRaftPeers(t, nodeIDs)
	nodes := map[string]*raftsvc.Service{}
	dataRoot := t.TempDir()

	for _, id := range nodeIDs {
		nodes[id] = newStartedTestRaftClusterNode(t, id, peers[id], peers, filepath.Join(dataRoot, id, "dragonboat"))
	}
	waitRaftClusterMembers(t, nodes, len(nodeIDs))

	leaderID, leader := waitAnyRaftLeader(t, nodes, "")
	firstApp := deployAppFixture("demo", "default")
	if err := leader.ApplyDeployApp(ctx, firstApp); err != nil {
		t.Fatal(err)
	}
	waitDeployAppOnRaftNodes(t, nodes, firstApp.Metadata)

	stopCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	if err := leader.Stop(stopCtx); err != nil {
		cancel()
		t.Fatal(err)
	}
	cancel()
	delete(nodes, leaderID)

	newLeaderID, newLeader := waitAnyRaftLeader(t, nodes, leaderID)
	if newLeaderID == leaderID {
		t.Fatalf("leader did not move after stop: %s", leaderID)
	}
	secondApp := deployAppFixture("after-failover", "default")
	if err := newLeader.ApplyDeployApp(ctx, secondApp); err != nil {
		t.Fatal(err)
	}
	waitDeployAppOnRaftNodes(t, nodes, secondApp.Metadata)
}

func reserveTestRaftPeers(t *testing.T, nodeIDs []string) map[string]string {
	t.Helper()
	peers := make(map[string]string, len(nodeIDs))
	for _, id := range nodeIDs {
		peers[id] = reserveTestRaftAddr(t)
	}
	return peers
}

func newStartedTestRaftClusterNode(tb testing.TB, id, bind string, peers map[string]string, dataDir string) *raftsvc.Service {
	tb.Helper()
	cfg := config.Default()
	cfg.Raft.Bind = bind
	cfg.Raft.Advertise = ""
	cfg.Raft.Bootstrap = true
	cfg.Raft.Peers = maps.Clone(peers)
	cfg.Raft.Data.Dir = dataDir

	svc := raftsvc.New(cfg, testLogger(), nodeid.Local{Value: id})
	if err := svc.Start(context.Background()); err != nil {
		tb.Fatal(err)
	}
	tb.Cleanup(func() {
		stopCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := svc.Stop(stopCtx); err != nil {
			tb.Logf("stop raft cluster node %s: %v", id, err)
		}
	})
	return svc
}

func waitAnyRaftLeader(tb testing.TB, nodes map[string]*raftsvc.Service, previousLeader string) (string, *raftsvc.Service) {
	tb.Helper()
	deadline := time.Now().Add(20 * time.Second)
	var last []string
	for time.Now().Before(deadline) {
		last = last[:0]
		for id, svc := range nodes {
			status, err := svc.Status(context.Background())
			if err != nil {
				last = append(last, fmt.Sprintf("%s: status error %v", id, err))
				continue
			}
			last = append(last, fmt.Sprintf("%s: %s leader=%t ready=%t leader_id=%s", id, status.State, status.IsLeader, status.Ready, status.LeaderID))
			if status.Ready && status.IsLeader && id != previousLeader {
				return id, svc
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
	tb.Fatalf("raft leader did not stabilize: %s", strings.Join(last, "; "))
	return "", nil
}

func waitRaftClusterMembers(tb testing.TB, nodes map[string]*raftsvc.Service, want int) {
	tb.Helper()
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		allReady := true
		for _, svc := range nodes {
			members, err := svc.ListMembers(context.Background())
			if err != nil || members.Len() != want {
				allReady = false
				break
			}
		}
		if allReady {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	tb.Fatalf("raft cluster members did not reach %d", want)
}

func waitDeployAppOnRaftNodes(tb testing.TB, nodes map[string]*raftsvc.Service, meta deployv1.Metadata) {
	tb.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		allReady := true
		for _, svc := range nodes {
			app, ok := svc.GetDesiredDeployApp(meta)
			if !ok || app.Metadata.Name != meta.Name {
				allReady = false
				break
			}
		}
		if allReady {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	tb.Fatalf("deploy app %s/%s was not replicated to all raft nodes", meta.Namespace, meta.Name)
}
