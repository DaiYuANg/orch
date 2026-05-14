package docker_test

import (
	"testing"

	"github.com/arcgolabs/collectionx/list"
	"github.com/docker/docker/api/types/container"

	deployv1 "github.com/daiyuang/orch/internal/deploy/v1alpha1"
	runtimedocker "github.com/daiyuang/orch/internal/runtime/docker"
)

func TestContainerLabelsMergesDockerLabels(t *testing.T) {
	labels := runtimedocker.ContainerLabels(
		deployv1.Metadata{Name: "app", Namespace: "demo"},
		deployv1.Workload{
			Name:    "api",
			Runtime: deployv1.RuntimeDocker,
			Run: deployv1.RunSpec{
				Options: deployv1.RunOptions{
					Docker: &deployv1.DockerOptions{
						Labels: map[string]string{
							"team":                    "platform",
							"custom.orch.io/workload": "ignored",
							"orch.io/app":             "ignored",
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

func TestWorkloadLabelsMatch(t *testing.T) {
	meta := deployv1.Metadata{Name: "app", Namespace: "demo"}
	workload := deployv1.Workload{Name: "api", Runtime: deployv1.RuntimeDocker}
	labels := runtimedocker.ContainerLabels(meta, workload)

	if !runtimedocker.WorkloadLabelsMatch(labels, meta, workload) {
		t.Fatal("expected labels to match app/workload identity")
	}
	if runtimedocker.WorkloadLabelsMatch(labels, deployv1.Metadata{Name: "other", Namespace: "demo"}, workload) {
		t.Fatal("expected app mismatch to fail")
	}
	if runtimedocker.WorkloadLabelsMatch(labels, meta, deployv1.Workload{Name: "other", Runtime: deployv1.RuntimeDocker}) {
		t.Fatal("expected workload mismatch to fail")
	}
}

type fakeWorkloadDNS struct{}

func (fakeWorkloadDNS) WorkloadNameserver() (string, bool) {
	return "172.17.0.1", true
}

func (fakeWorkloadDNS) WorkloadSearchDomains(namespace string) *list.List[string] {
	return list.NewList(namespace+".svc.orch.local", "svc.orch.local", "orch.local")
}

func TestApplyWorkloadDNS(t *testing.T) {
	hostCfg := &container.HostConfig{}
	runtimedocker.ApplyWorkloadDNS(hostCfg, fakeWorkloadDNS{}, "demo")

	if len(hostCfg.DNS) != 1 || hostCfg.DNS[0] != "172.17.0.1" {
		t.Fatalf("DNS = %#v", hostCfg.DNS)
	}
	wantSearch := []string{"demo.svc.orch.local", "svc.orch.local", "orch.local"}
	if len(hostCfg.DNSSearch) != len(wantSearch) {
		t.Fatalf("DNSSearch = %#v", hostCfg.DNSSearch)
	}
	for i := range wantSearch {
		if hostCfg.DNSSearch[i] != wantSearch[i] {
			t.Fatalf("DNSSearch = %#v", hostCfg.DNSSearch)
		}
	}
}
