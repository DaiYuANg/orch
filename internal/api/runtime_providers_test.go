package api_test

import (
	"context"
	"testing"

	"github.com/arcgolabs/collectionx/list"

	"github.com/lyonbrown4d/orch/internal/api"
	deployv1 "github.com/lyonbrown4d/orch/internal/deploy/v1alpha1"
	orchruntime "github.com/lyonbrown4d/orch/internal/runtime"
)

func TestRuntimeProvidersEndpointMapsProviderStatus(t *testing.T) {
	t.Parallel()

	manager := runtimeManagerWithStatuses(orchruntime.ProviderStatus{
		Kind:       deployv1.RuntimeProcess,
		Policy:     orchruntime.ProviderPolicyAuto,
		Available:  true,
		Registered: true,
		Status:     orchruntime.ProviderStatusRegistered,
	})

	out, err := api.NewRuntimeProvidersEndpoint(manager).Handle(context.Background(), &api.EmptyInput{})
	if err != nil {
		t.Fatal(err)
	}
	if out.Body.Items.Len() != 1 {
		t.Fatalf("items = %#v, want one provider", out.Body.Items)
	}
	got, ok := out.Body.Items.Get(0)
	if !ok || got.Kind != deployv1.RuntimeProcess || !got.Registered || got.Status != orchruntime.ProviderStatusRegistered {
		t.Fatalf("runtime provider item = %#v", got)
	}
}

func TestDiagnosticsEndpointIncludesRuntimeProviderSummary(t *testing.T) {
	t.Parallel()

	manager := runtimeManagerWithStatuses(
		orchruntime.ProviderStatus{
			Kind:       deployv1.RuntimeProcess,
			Policy:     orchruntime.ProviderPolicyAuto,
			Available:  true,
			Registered: true,
			Status:     orchruntime.ProviderStatusRegistered,
		},
		orchruntime.ProviderStatus{
			Kind:      deployv1.RuntimeDocker,
			Policy:    orchruntime.ProviderPolicyAuto,
			Available: true,
			Status:    orchruntime.ProviderStatusMissing,
		},
	)

	out, err := api.NewDiagnosticsEndpoint(nil, manager).Handle(context.Background(), &api.EmptyInput{})
	if err != nil {
		t.Fatal(err)
	}
	if out.Body.Runtime.Available != 2 || out.Body.Runtime.Registered != 1 || out.Body.Runtime.Providers.Len() != 2 {
		t.Fatalf("runtime diagnostics = %#v, want available=2 registered=1 providers=2", out.Body.Runtime)
	}
}

func runtimeManagerWithStatuses(statuses ...orchruntime.ProviderStatus) *orchruntime.Manager {
	return orchruntime.NewManagerWithStatus(nil, list.NewList(statuses...))
}
