package runtime_test

import (
	"log/slog"
	"strings"
	"testing"

	"github.com/arcgolabs/dix"

	"github.com/lyonbrown4d/orch/internal/config"
	deployv1 "github.com/lyonbrown4d/orch/internal/deploy/v1alpha1"
	"github.com/lyonbrown4d/orch/internal/dnssvc"
	orchruntime "github.com/lyonbrown4d/orch/internal/runtime"
)

func TestRuntimeModuleRegistersEnvironmentSelectedProviders(t *testing.T) {
	manager := startRuntimeModule(t, config.Default(), orchruntime.Environment{
		Docker:         true,
		Process:        true,
		WindowsService: true,
	})

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

func TestRuntimeProviderPolicyCanDisableDetectedProvider(t *testing.T) {
	cfg := config.Default()
	cfg.Runtime.Providers = map[string]config.RuntimeProviderConfig{
		string(deployv1.RuntimeDocker): {Policy: config.RuntimeProviderPolicyDisabled},
	}
	manager := startRuntimeModule(t, cfg, orchruntime.Environment{
		Docker:  true,
		Process: true,
	})
	if manager.HasProvider(deployv1.RuntimeDocker) {
		t.Fatalf("docker provider registered despite disabled policy; got %v", manager.RegisteredKinds().Values())
	}
	status, ok := runtimeStatus(manager, deployv1.RuntimeDocker)
	if !ok {
		t.Fatalf("docker provider status missing")
	}
	if status.Status != orchruntime.ProviderStatusDisabled || status.Policy != orchruntime.ProviderPolicyDisabled {
		t.Fatalf("docker status = %#v, want disabled policy/status", status)
	}
}

func TestRuntimeProviderPolicyRequiredFailsWhenUnavailable(t *testing.T) {
	cfg := config.Default()
	cfg.Runtime.Providers = map[string]config.RuntimeProviderConfig{
		string(deployv1.RuntimeDocker): {Policy: config.RuntimeProviderPolicyRequired},
	}
	logger := slog.New(slog.DiscardHandler)
	app := newRuntimeApp(t, cfg, logger, orchruntime.Environment{Process: true})
	_, err := app.Start(t.Context())
	if err != nil {
		if strings.Contains(err.Error(), "required runtime provider docker is unavailable") {
			return
		}
		t.Fatalf("start error = %v, want required docker unavailable", err)
	}
	t.Fatal("start succeeded, want required docker unavailable error")
}

func startRuntimeModule(t *testing.T, cfg config.Config, env orchruntime.Environment) *orchruntime.Manager {
	t.Helper()
	logger := slog.New(slog.DiscardHandler)
	app := newRuntimeApp(t, cfg, logger, env)
	rt, err := app.Start(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	manager, err := dix.ResolveAs[*orchruntime.Manager](rt.Container())
	if err != nil {
		t.Fatal(err)
	}
	return manager
}

func newRuntimeApp(t *testing.T, cfg config.Config, logger *slog.Logger, env orchruntime.Environment) *dix.App {
	t.Helper()
	return dix.New(
		"runtime-module-test",
		dix.Modules(
			dix.NewModule(
				"runtime-test-deps",
				dix.Providers(
					dix.Value(cfg),
					dix.Value(logger),
					dix.Provider0(func() *dnssvc.Service {
						return dnssvc.New(cfg, logger)
					}),
				),
			),
			orchruntime.ModuleWithEnvironment(env),
		),
	)
}

func runtimeStatus(manager *orchruntime.Manager, kind deployv1.RuntimeKind) (orchruntime.ProviderStatus, bool) {
	var out orchruntime.ProviderStatus
	found := false
	manager.ProviderStatuses().Range(func(_ int, status orchruntime.ProviderStatus) bool {
		if status.Kind != kind {
			return true
		}
		out = status
		found = true
		return false
	})
	return out, found
}
