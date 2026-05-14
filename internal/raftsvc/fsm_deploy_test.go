package raftsvc_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	deployv1 "github.com/lyonbrown4d/orch/internal/deploy/v1alpha1"
	"github.com/lyonbrown4d/orch/internal/workloadmeta"
)

func TestRaftApplyDeployApp(t *testing.T) {
	svc := newStartedTestRaft(t, "node-fsm-app")
	waitRaftLeader(t, svc)

	app := deployAppFixture("demo", "ns1")
	if err := svc.ApplyDeployApp(context.Background(), app); err != nil {
		t.Fatal(err)
	}

	apps := svc.ListDesiredDeployApps()
	got, ok := apps.Get(0)
	if apps.Len() != 1 || !ok || got.Metadata.Name != "demo" {
		t.Fatalf("list = %#v", apps)
	}
}

func TestRaftDeployAppDefaultNamespaceDelete(t *testing.T) {
	svc := newStartedTestRaft(t, "node-fsm-delete")
	waitRaftLeader(t, svc)

	app := deployAppFixture("demo", "")
	if err := svc.ApplyDeployApp(context.Background(), app); err != nil {
		t.Fatal(err)
	}

	meta := deployv1.Metadata{Name: "demo", Namespace: "default"}
	if _, ok := svc.GetDesiredDeployApp(meta); !ok {
		t.Fatal("default namespace app not found")
	}
	if err := svc.ApplyDeleteDeployApp(context.Background(), meta); err != nil {
		t.Fatal(err)
	}

	if apps := svc.ListDesiredDeployApps(); apps.Len() != 0 {
		t.Fatalf("apps after delete = %#v", apps.Values())
	}
}

func TestRaftDeploySnapshotRoundTrip(t *testing.T) {
	ctx := context.Background()
	dataDir := filepath.Join(t.TempDir(), "dragonboat")
	raftAddr := reserveTestRaftAddr(t)
	app := deployAppFixture("demo", "default")

	first := newStartedTestRaftWithDataDir(t, "node-fsm-snapshot", true, raftAddr, dataDir)
	waitRaftLeader(t, first)
	if err := first.ApplyDeployApp(ctx, app); err != nil {
		t.Fatal(err)
	}
	stopCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	if err := first.Stop(stopCtx); err != nil {
		cancel()
		t.Fatal(err)
	}
	cancel()

	second := newStartedTestRaftWithDataDir(t, "node-fsm-snapshot", true, raftAddr, dataDir)
	waitRaftLeader(t, second)
	got, ok := second.GetDesiredDeployApp(deployv1.Metadata{Name: "demo", Namespace: "default"})
	if !ok || got.Metadata.Name != "demo" {
		t.Fatalf("after restore = %#v ok=%t", got, ok)
	}
}

func TestRaftApplyWorkloadAssignment(t *testing.T) {
	svc := newStartedTestRaft(t, "node-fsm-assignment")
	waitRaftLeader(t, svc)

	assignment := assignmentFixture(workloadmeta.AssignmentStatusRunning)
	if err := svc.ApplyWorkloadAssignment(context.Background(), assignment); err != nil {
		t.Fatal(err)
	}

	got, ok := svc.GetWorkloadAssignment(workloadmeta.AssignmentKey(assignment.Metadata, assignment.Workload))
	if !ok {
		t.Fatal("assignment not stored")
	}
	if got.Node != "node-a" || got.Status != workloadmeta.AssignmentStatusRunning || got.Artifact != "nginx" {
		t.Fatalf("assignment = %#v", got)
	}
}

func TestRaftAssignmentSnapshotRoundTrip(t *testing.T) {
	ctx := context.Background()
	dataDir := filepath.Join(t.TempDir(), "dragonboat")
	raftAddr := reserveTestRaftAddr(t)
	assignment := assignmentFixture(workloadmeta.AssignmentStatusAssigned)

	first := newStartedTestRaftWithDataDir(t, "node-fsm-assignment-snapshot", true, raftAddr, dataDir)
	waitRaftLeader(t, first)
	if err := first.ApplyWorkloadAssignment(ctx, assignment); err != nil {
		t.Fatal(err)
	}
	stopCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	if err := first.Stop(stopCtx); err != nil {
		cancel()
		t.Fatal(err)
	}
	cancel()

	second := newStartedTestRaftWithDataDir(t, "node-fsm-assignment-snapshot", true, raftAddr, dataDir)
	waitRaftLeader(t, second)
	got, ok := second.GetWorkloadAssignment(workloadmeta.AssignmentKey(assignment.Metadata, assignment.Workload))
	if !ok {
		t.Fatal("assignment not restored")
	}
	if got.Node != "node-a" || got.Status != workloadmeta.AssignmentStatusAssigned {
		t.Fatalf("after restore = %#v", got)
	}
}

func deployAppFixture(name, namespace string) deployv1.App {
	return deployv1.App{
		Metadata: deployv1.Metadata{Name: name, Namespace: namespace},
		Workloads: []deployv1.Workload{{
			Name:    "w",
			Runtime: deployv1.RuntimeDocker,
		}},
	}
}

func assignmentFixture(status string) workloadmeta.Assignment {
	meta := deployv1.Metadata{Name: "demo", Namespace: "ns1"}
	return workloadmeta.Assignment{
		Metadata: meta,
		Workload: "web",
		Node:     "node-a",
		Runtime:  deployv1.RuntimeDocker,
		Artifact: "nginx",
		Status:   status,
	}
}
