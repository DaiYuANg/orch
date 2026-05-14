package raftsvc_test

import (
	"context"
	"testing"

	deployv1 "github.com/daiyuang/orch/internal/deploy/v1alpha1"
)

func BenchmarkRaftApplyDeployApp(b *testing.B) {
	svc := newStartedTestRaft(b, "node-bench-apply")
	waitRaftLeader(b, svc)

	app := benchmarkDeployApp("bench")
	b.ReportAllocs()
	b.ResetTimer()

	for range b.N {
		if err := svc.ApplyDeployApp(context.Background(), app); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkRaftStatus(b *testing.B) {
	svc := newStartedTestRaft(b, "node-bench-status")
	waitRaftLeader(b, svc)

	ctx := context.Background()
	b.ReportAllocs()
	b.ResetTimer()

	for range b.N {
		if _, err := svc.Status(ctx); err != nil {
			b.Fatal(err)
		}
	}
}

func benchmarkDeployApp(name string) deployv1.App {
	return deployv1.App{
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
}
