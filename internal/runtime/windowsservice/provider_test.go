package windowsservice_test

import (
	"testing"

	deployv1 "github.com/daiyuang/orch/internal/deploy/v1alpha1"
	"github.com/daiyuang/orch/internal/runtime/windowsservice"
)

func TestDefaultServiceName(t *testing.T) {
	t.Parallel()

	meta := deployv1.Metadata{Name: "Demo_App", Namespace: ""}
	if got := windowsservice.DefaultServiceName(meta, "api"); got != "orch-default-demo_app-api" {
		t.Fatalf("service name = %q", got)
	}
	if got := windowsservice.NormalizeServiceName("custom api/service"); got != "custom-api-service" {
		t.Fatalf("custom service name = %q", got)
	}
}

func TestWorkloadDisplayName(t *testing.T) {
	t.Parallel()

	provider := windowsservice.NewProvider(nil, nil)
	meta := deployv1.Metadata{Name: "demo", Namespace: "prod"}
	workload := deployv1.Workload{Name: "api"}
	if got := provider.WorkloadDisplayName(meta, workload); got != "orch prod/demo/api" {
		t.Fatalf("display name = %q", got)
	}
	workload.Run.Options.WindowsService = &deployv1.WindowsServiceOptions{DisplayName: "Demo API"}
	if got := provider.WorkloadDisplayName(meta, workload); got != "Demo API" {
		t.Fatalf("display override = %q", got)
	}
}
