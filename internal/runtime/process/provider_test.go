package process

import (
	"context"
	"io"
	"log/slog"
	"os"
	"testing"
	"time"

	deployv1 "github.com/daiyuang/orch/internal/deploy/v1alpha1"
)

func TestProviderDeployStop(t *testing.T) {
	if os.Getenv("ORCH_PROCESS_HELPER") == "1" {
		time.Sleep(time.Minute)
		return
	}

	provider := &Provider{
		logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
		root:   t.TempDir(),
	}
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	meta := deployv1.Metadata{Name: "demo", Namespace: "default"}
	workload := deployv1.Workload{
		Name:    "worker",
		Runtime: deployv1.RuntimeProcess,
		Run: deployv1.RunSpec{
			Exec: deployv1.ExecSpec{
				Command: []string{exe, "-test.run=TestProviderDeployStop"},
			},
			Env: []deployv1.EnvVar{
				{Name: "ORCH_PROCESS_HELPER", Value: "1"},
			},
			Options: deployv1.RunOptions{
				Process: &deployv1.ProcessOptions{GracefulStopTimeout: "10ms"},
			},
		},
	}

	if err := provider.Deploy(context.Background(), meta, workload); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(provider.statePath(meta, workload.Name)); err != nil {
		t.Fatalf("state file: %v", err)
	}
	if err := provider.Stop(context.Background(), meta, workload.Name); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(provider.statePath(meta, workload.Name)); !os.IsNotExist(err) {
		t.Fatalf("state file after stop error = %v, want not exist", err)
	}
}
