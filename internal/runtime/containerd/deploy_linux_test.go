//go:build linux

package containerd

import (
	"path/filepath"
	"slices"
	"testing"

	"github.com/arcgolabs/collectionx/list"
	runtimeapi "k8s.io/cri-api/pkg/apis/runtime/v1"

	deployv1 "github.com/lyonbrown4d/orch/internal/deploy/v1alpha1"
)

type fakeDNS struct{}

func (fakeDNS) WorkloadNameserver() (string, bool) {
	return "10.0.0.53", true
}

func (fakeDNS) WorkloadSearchDomains(namespace string) *list.List[string] {
	return list.NewList(namespace+".svc.orch.local", "svc.orch.local")
}

func TestCRIDNSConfig(t *testing.T) {
	t.Parallel()

	got := criDNSConfig(fakeDNS{}, "demo")
	if got == nil {
		t.Fatal("expected DNS config")
	}
	if !slices.Equal(got.Servers, []string{"10.0.0.53"}) {
		t.Fatalf("servers = %#v", got.Servers)
	}
	if !slices.Equal(got.Searches, []string{"demo.svc.orch.local", "svc.orch.local"}) {
		t.Fatalf("searches = %#v", got.Searches)
	}
}

func TestCRIContainerConfig(t *testing.T) {
	t.Parallel()

	cfg := criContainerConfig("busybox:latest", deployv1.Metadata{Name: "demo", Namespace: "default"}, deployv1.Workload{
		Name:    "api",
		Runtime: deployv1.RuntimeContainerd,
		Run: deployv1.RunSpec{
			Exec: deployv1.ExecSpec{
				Command: []string{"sleep"},
				Args:    []string{"60"},
			},
			Env: []deployv1.EnvVar{{Name: "APP_ENV", Value: "test"}},
		},
		Resources: &deployv1.Resources{CPUMillis: 500, MemoryBytes: 128 << 20},
	})

	if cfg.Image.GetImage() != "busybox:latest" {
		t.Fatalf("image = %q", cfg.Image.GetImage())
	}
	if !slices.Equal(cfg.Command, []string{"sleep"}) || !slices.Equal(cfg.Args, []string{"60"}) {
		t.Fatalf("argv = %#v %#v", cfg.Command, cfg.Args)
	}
	if len(cfg.Envs) != 1 || cfg.Envs[0].Key != "APP_ENV" || cfg.Envs[0].Value != "test" {
		t.Fatalf("envs = %#v", cfg.Envs)
	}
	res := cfg.GetLinux().GetResources()
	if res == nil {
		t.Fatal("expected Linux resources")
	}
	if res.GetMemoryLimitInBytes() != 128<<20 {
		t.Fatalf("memory = %d", res.GetMemoryLimitInBytes())
	}
	if res.GetCpuQuota() == 0 || res.GetCpuPeriod() == 0 {
		t.Fatalf("cpu quota/period = %d/%d", res.GetCpuQuota(), res.GetCpuPeriod())
	}
}

func TestCRISandboxConfigWithoutResolver(t *testing.T) {
	t.Parallel()

	provider := &Provider{root: t.TempDir(), dns: nil}
	cfg := criSandboxConfig(provider, deployv1.Metadata{Name: "demo", Namespace: "prod"}, deployv1.Workload{Name: "api"})
	if cfg.GetDnsConfig() != nil {
		t.Fatalf("dns config = %#v, want nil without resolver", cfg.GetDnsConfig())
	}
	if cfg.GetMetadata().GetNamespace() != "prod" {
		t.Fatalf("namespace = %q", cfg.GetMetadata().GetNamespace())
	}
	if cfg.GetLogDirectory() == "" {
		t.Fatal("expected log directory")
	}
}

func TestCRIWorkloadLabelSelector(t *testing.T) {
	t.Parallel()

	got := criWorkloadLabelSelector(deployv1.Metadata{Namespace: "prod"}, " api ")
	if got["orch.io/namespace"] != "prod" {
		t.Fatalf("namespace label = %q", got["orch.io/namespace"])
	}
	if got["orch.io/workload"] != "api" {
		t.Fatalf("workload label = %q", got["orch.io/workload"])
	}
}

func TestCRIContainerStateStatus(t *testing.T) {
	t.Parallel()

	tests := map[runtimeapi.ContainerState]string{
		runtimeapi.ContainerState_CONTAINER_CREATED: "created",
		runtimeapi.ContainerState_CONTAINER_RUNNING: "running",
		runtimeapi.ContainerState_CONTAINER_EXITED:  "exited",
		runtimeapi.ContainerState_CONTAINER_UNKNOWN: "unknown",
	}
	for state, want := range tests {
		if got := criContainerStateStatus(state); got != want {
			t.Fatalf("state %s = %q, want %q", state, got, want)
		}
	}
}

func TestCRIContainerLogPath(t *testing.T) {
	t.Parallel()

	meta := deployv1.Metadata{Name: "demo", Namespace: "prod"}
	provider := &Provider{root: t.TempDir()}

	got := criContainerLogPath(provider, meta, "api", &runtimeapi.ContainerStatus{LogPath: "custom.log"})
	want := filepath.Join(provider.logDir(meta, "api"), "custom.log")
	if got != want {
		t.Fatalf("relative log path = %q, want %q", got, want)
	}

	got = criContainerLogPath(provider, meta, "api", &runtimeapi.ContainerStatus{LogPath: "/var/log/orch/api.log"})
	if got != "/var/log/orch/api.log" {
		t.Fatalf("absolute log path = %q", got)
	}

	got = criContainerLogPath(provider, meta, "api", &runtimeapi.ContainerStatus{})
	want = filepath.Join(provider.logDir(meta, "api"), "api.log")
	if got != want {
		t.Fatalf("fallback log path = %q, want %q", got, want)
	}
}
