package docker

import (
	"testing"

	deployv1 "github.com/daiyuang/orch/internal/deploy/v1alpha1"
)

func TestContainerLabelsMergesDockerLabels(t *testing.T) {
	labels := containerLabels(
		deployv1.Metadata{Name: "app", Namespace: "demo"},
		deployv1.Workload{
			Name:    "api",
			Runtime: deployv1.RuntimeDocker,
			Run: deployv1.RunSpec{
				Options: deployv1.RunOptions{
					Docker: &deployv1.DockerOptions{
						Labels: map[string]string{
							"team":              "platform",
							" orch.io/workload": "ignored",
							"orch.io/app":       "ignored",
						},
					},
				},
			},
		},
	)

	if labels["team"] != "platform" {
		t.Fatalf("team label = %q", labels["team"])
	}
	if labels["orch.io/app"] != "app" {
		t.Fatalf("orch.io/app label = %q", labels["orch.io/app"])
	}
	if labels["orch.io/namespace"] != "demo" {
		t.Fatalf("orch.io/namespace label = %q", labels["orch.io/namespace"])
	}
	if labels["orch.io/workload"] != "api" {
		t.Fatalf("orch.io/workload label = %q", labels["orch.io/workload"])
	}
}
