package raftsvc

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	deployv1 "github.com/daiyuang/orch/internal/deploy/v1alpha1"
)

func BenchmarkRaftApplyDeployApp(b *testing.B) {
	svc := newStartedTestRaft(b, "node-bench-apply", true)
	waitRaftLeader(b, svc)

	payload := benchmarkDeployAppCommand(b, "bench")
	b.SetBytes(int64(len(payload)))
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		if err := svc.applyCommand(payload, 30*time.Second, "not leader"); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkRaftStatus(b *testing.B) {
	svc := newStartedTestRaft(b, "node-bench-status", true)
	waitRaftLeader(b, svc)

	ctx := context.Background()
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		if _, err := svc.Status(ctx); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkFSMApplyDeployApp(b *testing.B) {
	f := &schedulingFSM{}
	payload := benchmarkDeployAppCommand(b, "bench")

	b.SetBytes(int64(len(payload)))
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		f.applyCommandPayload(payload)
	}
}

func benchmarkDeployAppCommand(tb testing.TB, name string) []byte {
	tb.Helper()
	app := deployv1.App{
		APIVersion: "orch/v1alpha1",
		Kind:       "App",
		Metadata: deployv1.Metadata{
			Name:      name,
			Namespace: "bench",
		},
		Workloads: []deployv1.Workload{
			{
				Name:     "web",
				Kind:     deployv1.WorkloadKindService,
				Runtime:  deployv1.RuntimeDocker,
				Replicas: 1,
				Run: deployv1.RunSpec{
					Artifact: deployv1.ArtifactSpec{Image: "nginx:alpine"},
				},
			},
		},
	}
	payload, err := json.Marshal(struct {
		Type string       `json:"type"`
		App  deployv1.App `json:"app"`
	}{
		Type: cmdUpsertDeployApp,
		App:  app,
	})
	if err != nil {
		tb.Fatal(err)
	}
	return payload
}
