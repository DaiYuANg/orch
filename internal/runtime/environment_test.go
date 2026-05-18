package runtime_test

import (
	"log/slog"
	"testing"

	"github.com/arcgolabs/dix"

	"github.com/lyonbrown4d/orch/internal/config"
	deployv1 "github.com/lyonbrown4d/orch/internal/deploy/v1alpha1"
	"github.com/lyonbrown4d/orch/internal/dnssvc"
	orchruntime "github.com/lyonbrown4d/orch/internal/runtime"
)

func TestRuntimeModuleRegistersEnvironmentSelectedProviders(t *testing.T) {
	logger := slog.New(slog.DiscardHandler)
	app := dix.New(
		"runtime-module-test",
		dix.Modules(
			dix.NewModule(
				"runtime-test-deps",
				dix.Providers(
					dix.Value(logger),
					dix.Provider0(func() *dnssvc.Service {
						return dnssvc.New(config.Default(), logger)
					}),
				),
			),
			orchruntime.ModuleWithEnvironment(orchruntime.Environment{
				Docker:         true,
				Process:        true,
				WindowsService: true,
			}),
		),
	)
	rt, err := app.Start(t.Context())
	if err != nil {
		t.Fatal(err)
	}

	manager, err := dix.ResolveAs[*orchruntime.Manager](rt.Container())
	if err != nil {
		t.Fatal(err)
	}
	for _, kind := range []deployv1.RuntimeKind{
		deployv1.RuntimeDocker,
		deployv1.RuntimeProcess,
		deployv1.RuntimeWindowsService,
	} {
		if !manager.HasProvider(kind) {
			t.Fatalf("provider %s not registered; got %v", kind, manager.RegisteredKinds().Values())
		}
	}
	if manager.HasProvider(deployv1.RuntimeContainerd) || manager.HasProvider(deployv1.RuntimeSystemd) {
		t.Fatalf("environment-disabled providers were registered: %v", manager.RegisteredKinds().Values())
	}
}
