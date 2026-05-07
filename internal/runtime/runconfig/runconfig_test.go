package runconfig

import (
	"math"
	"slices"
	"testing"

	"github.com/arcgolabs/collectionx/list"

	deployv1 "github.com/daiyuang/orch/internal/deploy/v1alpha1"
)

func TestEnv(t *testing.T) {
	got := Env(list.NewList(
		deployv1.EnvVar{Name: " PORT ", Value: "8080"},
		deployv1.EnvVar{Name: "", Value: "skip"},
		deployv1.EnvVar{Name: "EMPTY"},
	))
	want := []string{"PORT=8080", "EMPTY="}
	if !slices.Equal(got.Values(), want) {
		t.Fatalf("Env() = %#v, want %#v", got.Values(), want)
	}
}

func TestCommandArgs(t *testing.T) {
	got := CommandArgs(deployv1.RunSpec{
		Exec: deployv1.ExecSpec{
			Command: []string{"/bin/server"},
			Args:    []string{"--port", "8080"},
		},
	})
	want := []string{"/bin/server", "--port", "8080"}
	if !slices.Equal(got.Values(), want) {
		t.Fatalf("CommandArgs() = %#v, want %#v", got.Values(), want)
	}
}

func TestProcessCommand(t *testing.T) {
	exe, args, ok := ProcessCommand(deployv1.RunSpec{
		Exec: deployv1.ExecSpec{
			Command: []string{"/bin/server", "serve"},
			Args:    []string{"--port", "8080"},
		},
	})
	if !ok || exe != "/bin/server" || !slices.Equal(args.Values(), []string{"serve", "--port", "8080"}) {
		t.Fatalf("ProcessCommand() = %q %#v %v", exe, args.Values(), ok)
	}

	exe, args, ok = ProcessCommand(deployv1.RunSpec{
		Artifact: deployv1.ArtifactSpec{Path: "/opt/app/api"},
		Exec:     deployv1.ExecSpec{Args: []string{"--port", "8080"}},
	})
	if !ok || exe != "/opt/app/api" || !slices.Equal(args.Values(), []string{"--port", "8080"}) {
		t.Fatalf("ProcessCommand(path) = %q %#v %v", exe, args.Values(), ok)
	}
}

func TestArtifactSummary(t *testing.T) {
	got := ArtifactSummary(deployv1.RunSpec{Artifact: deployv1.ArtifactSpec{Image: "nginx", Path: "/ignored"}})
	if got != "nginx" {
		t.Fatalf("ArtifactSummary(image) = %q", got)
	}
	got = ArtifactSummary(deployv1.RunSpec{Artifact: deployv1.ArtifactSpec{Path: "/opt/app/api"}})
	if got != "/opt/app/api" {
		t.Fatalf("ArtifactSummary(path) = %q", got)
	}
	got = ArtifactSummary(deployv1.RunSpec{Exec: deployv1.ExecSpec{Command: []string{"/opt/app/worker"}}})
	if got != "/opt/app/worker" {
		t.Fatalf("ArtifactSummary(command) = %q", got)
	}
}

func TestNanoCPUs(t *testing.T) {
	if got := NanoCPUs(1500); got != 1_500_000_000 {
		t.Fatalf("NanoCPUs(1500) = %d", got)
	}
	if got := NanoCPUs(math.MaxInt64); got != math.MaxInt64 {
		t.Fatalf("NanoCPUs(MaxInt64) = %d", got)
	}
}

func TestCFSQuota(t *testing.T) {
	quota, period := CFSQuota(250)
	if quota != 25_000 || period != 100_000 {
		t.Fatalf("CFSQuota(250) = (%d, %d), want (25000, 100000)", quota, period)
	}
}
