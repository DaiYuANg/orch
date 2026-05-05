package runconfig

import (
	"math"
	"reflect"
	"testing"

	deployv1 "github.com/daiyuang/orch/internal/deploy/v1alpha1"
)

func TestEnv(t *testing.T) {
	got := Env([]deployv1.EnvVar{
		{Name: " PORT ", Value: "8080"},
		{Name: "", Value: "skip"},
		{Name: "EMPTY"},
	})
	want := []string{"PORT=8080", "EMPTY="}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Env() = %#v, want %#v", got, want)
	}
}

func TestCommandArgs(t *testing.T) {
	got := CommandArgs(deployv1.RunSpec{
		Command: []string{"/bin/server"},
		Args:    []string{"--port", "8080"},
	})
	want := []string{"/bin/server", "--port", "8080"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("CommandArgs() = %#v, want %#v", got, want)
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
