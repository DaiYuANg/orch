package runtime

import (
	"fmt"
	"log/slog"

	"github.com/arcgolabs/collectionx/list"
	"github.com/arcgolabs/dix"
	"github.com/samber/mo"

	"github.com/lyonbrown4d/orch/internal/config"
	deployv1 "github.com/lyonbrown4d/orch/internal/deploy/v1alpha1"
	"github.com/lyonbrown4d/orch/internal/dnssvc"
	runtimecontainerd "github.com/lyonbrown4d/orch/internal/runtime/containerd"
	runtimedocker "github.com/lyonbrown4d/orch/internal/runtime/docker"
	runtimefirecracker "github.com/lyonbrown4d/orch/internal/runtime/firecracker"
	runtimepodman "github.com/lyonbrown4d/orch/internal/runtime/podman"
	runtimeprocess "github.com/lyonbrown4d/orch/internal/runtime/process"
	runtimesystemd "github.com/lyonbrown4d/orch/internal/runtime/systemd"
	runtimewindowsservice "github.com/lyonbrown4d/orch/internal/runtime/windowsservice"
)

type providerList []Provider

type runtimeProviderDeps struct {
	config      config.Config
	environment Environment
	logger      *slog.Logger
	dns         *dnssvc.Service
}

type optionalProviderGroupBase struct {
	docker      dix.Conditional[*runtimedocker.Provider]
	podman      dix.Conditional[*runtimepodman.Provider]
	containerd  dix.Conditional[*runtimecontainerd.Provider]
	firecracker dix.Conditional[*runtimefirecracker.Provider]
	process     dix.Conditional[*runtimeprocess.Provider]
	systemd     dix.Conditional[*runtimesystemd.Provider]
}

type optionalProviderGroup struct {
	optionalProviderGroupBase
	windowsService dix.Conditional[*runtimewindowsservice.Provider]
}

func Module() dix.Module {
	return moduleWithEnvironmentProvider(dix.Provider0(DetectEnvironment, dix.Eager()))
}

func ModuleWithEnvironment(env Environment) dix.Module {
	return moduleWithEnvironmentProvider(dix.Value(env))
}

func moduleWithEnvironmentProvider(environmentProvider dix.ProviderFunc) dix.Module {
	return dix.NewModule(
		"runtime",
		dix.Providers(
			environmentProvider,
			dix.Provider4(newRuntimeProviderDeps),
			dix.ConditionalProviderErr1(optionalDockerProvider, dix.Eager()),
			dix.ConditionalProviderErr1(optionalPodmanProvider, dix.Eager()),
			dix.ConditionalProviderErr1(optionalContainerdProvider, dix.Eager()),
			dix.ConditionalProviderErr1(optionalFirecrackerProvider, dix.Eager()),
			dix.ConditionalProviderErr1(optionalProcessProvider, dix.Eager()),
			dix.ConditionalProviderErr1(optionalSystemdProvider, dix.Eager()),
			dix.ConditionalProviderErr1(optionalWindowsServiceProvider, dix.Eager()),
			dix.Provider6(func(
				dockerProvider dix.Conditional[*runtimedocker.Provider],
				podmanProvider dix.Conditional[*runtimepodman.Provider],
				containerdProvider dix.Conditional[*runtimecontainerd.Provider],
				firecrackerProvider dix.Conditional[*runtimefirecracker.Provider],
				processProvider dix.Conditional[*runtimeprocess.Provider],
				systemdProvider dix.Conditional[*runtimesystemd.Provider],
			) optionalProviderGroupBase {
				return optionalProviderGroupBase{
					docker:      dockerProvider,
					podman:      podmanProvider,
					containerd:  containerdProvider,
					firecracker: firecrackerProvider,
					process:     processProvider,
					systemd:     systemdProvider,
				}
			}, dix.Eager()),
			dix.Provider2(func(
				base optionalProviderGroupBase,
				windowsServiceProvider dix.Conditional[*runtimewindowsservice.Provider],
			) optionalProviderGroup {
				return optionalProviderGroup{
					optionalProviderGroupBase: base,
					windowsService:            windowsServiceProvider,
				}
			}, dix.Eager()),
			dix.Provider1(providerListFromConditionals, dix.Eager()),
			dix.Provider3(newProviderStatuses, dix.Eager()),
			dix.Provider3(func(logger *slog.Logger, providers providerList, statuses *list.List[ProviderStatus]) *Manager {
				return NewManagerWithStatus(logger, statuses, providers...)
			}, dix.Eager()),
		),
	)
}

func newRuntimeProviderDeps(cfg config.Config, env Environment, logger *slog.Logger, dns *dnssvc.Service) runtimeProviderDeps {
	return runtimeProviderDeps{
		config:      cfg,
		environment: env,
		logger:      logger,
		dns:         dns,
	}
}

func providerListFromConditionals(group optionalProviderGroup) providerList {
	out := providerList{}
	if provider, ok := group.docker.Get(); ok {
		out = append(out, provider)
	}
	if provider, ok := group.podman.Get(); ok {
		out = append(out, provider)
	}
	if provider, ok := group.containerd.Get(); ok {
		out = append(out, provider)
	}
	if provider, ok := group.firecracker.Get(); ok {
		out = append(out, provider)
	}
	if provider, ok := group.process.Get(); ok {
		out = append(out, provider)
	}
	if provider, ok := group.systemd.Get(); ok {
		out = append(out, provider)
	}
	if provider, ok := group.windowsService.Get(); ok {
		out = append(out, provider)
	}
	return out
}

func optionalDockerProvider(deps runtimeProviderDeps) (dix.Conditional[*runtimedocker.Provider], error) {
	ok, err := shouldRegisterProvider(deps, deployv1.RuntimeDocker)
	if !ok || err != nil {
		return mo.None[*runtimedocker.Provider](), err
	}
	return mo.Some(runtimedocker.NewProvider(deps.logger, deps.dns)), nil
}

func optionalPodmanProvider(deps runtimeProviderDeps) (dix.Conditional[*runtimepodman.Provider], error) {
	ok, err := shouldRegisterProvider(deps, deployv1.RuntimePodman)
	if !ok || err != nil {
		return mo.None[*runtimepodman.Provider](), err
	}
	return mo.Some(runtimepodman.NewProvider(deps.logger, deps.dns)), nil
}

func optionalContainerdProvider(deps runtimeProviderDeps) (dix.Conditional[*runtimecontainerd.Provider], error) {
	ok, err := shouldRegisterProvider(deps, deployv1.RuntimeContainerd)
	if !ok || err != nil {
		return mo.None[*runtimecontainerd.Provider](), err
	}
	return mo.Some(runtimecontainerd.NewProvider(deps.logger, deps.dns)), nil
}

func optionalFirecrackerProvider(deps runtimeProviderDeps) (dix.Conditional[*runtimefirecracker.Provider], error) {
	ok, err := shouldRegisterProvider(deps, deployv1.RuntimeFirecracker)
	if !ok || err != nil {
		return mo.None[*runtimefirecracker.Provider](), err
	}
	return mo.Some(runtimefirecracker.NewProvider(deps.logger, deps.dns)), nil
}

func optionalProcessProvider(deps runtimeProviderDeps) (dix.Conditional[*runtimeprocess.Provider], error) {
	ok, err := shouldRegisterProvider(deps, deployv1.RuntimeProcess)
	if !ok || err != nil {
		return mo.None[*runtimeprocess.Provider](), err
	}
	return mo.Some(runtimeprocess.NewProvider(deps.logger, deps.dns)), nil
}

func optionalSystemdProvider(deps runtimeProviderDeps) (dix.Conditional[*runtimesystemd.Provider], error) {
	ok, err := shouldRegisterProvider(deps, deployv1.RuntimeSystemd)
	if !ok || err != nil {
		return mo.None[*runtimesystemd.Provider](), err
	}
	return mo.Some(runtimesystemd.NewProvider(deps.logger, deps.dns)), nil
}

func optionalWindowsServiceProvider(deps runtimeProviderDeps) (dix.Conditional[*runtimewindowsservice.Provider], error) {
	ok, err := shouldRegisterProvider(deps, deployv1.RuntimeWindowsService)
	if !ok || err != nil {
		return mo.None[*runtimewindowsservice.Provider](), err
	}
	return mo.Some(runtimewindowsservice.NewProvider(deps.logger, deps.dns)), nil
}

func shouldRegisterProvider(deps runtimeProviderDeps, kind deployv1.RuntimeKind) (bool, error) {
	policy := runtimeProviderPolicy(deps.config, kind)
	if policy == ProviderPolicyDisabled {
		return false, nil
	}
	if deps.environment.ProviderAvailable(kind) {
		return true, nil
	}
	if policy == ProviderPolicyRequired {
		return false, fmt.Errorf("required runtime provider %s is unavailable: %s", kind, deps.environment.ProviderUnavailableReason(kind))
	}
	return false, nil
}

func runtimeProviderPolicy(cfg config.Config, kind deployv1.RuntimeKind) ProviderPolicy {
	switch cfg.Runtime.ProviderPolicy(string(kind)) {
	case config.RuntimeProviderPolicyRequired:
		return ProviderPolicyRequired
	case config.RuntimeProviderPolicyDisabled:
		return ProviderPolicyDisabled
	default:
		return ProviderPolicyAuto
	}
}

func newProviderStatuses(cfg config.Config, env Environment, providers providerList) *list.List[ProviderStatus] {
	registered := make(map[deployv1.RuntimeKind]struct{}, len(providers))
	for _, provider := range providers {
		if provider == nil {
			continue
		}
		registered[provider.Kind()] = struct{}{}
	}

	out := list.NewList[ProviderStatus]()
	KnownProviderKinds().Range(func(_ int, kind deployv1.RuntimeKind) bool {
		_, isRegistered := registered[kind]
		policy := runtimeProviderPolicy(cfg, kind)
		available := env.ProviderAvailable(kind)
		status := ProviderStatusMissing
		reason := env.ProviderUnavailableReason(kind)
		if policy == ProviderPolicyDisabled {
			status = ProviderStatusDisabled
			reason = "disabled by runtime provider policy"
		} else if isRegistered {
			status = ProviderStatusRegistered
			reason = ""
		}
		out.Add(ProviderStatus{
			Kind:       kind,
			Policy:     policy,
			Available:  available,
			Registered: isRegistered,
			Status:     status,
			Reason:     reason,
		})
		return true
	})
	return sortProviderStatuses(out)
}
