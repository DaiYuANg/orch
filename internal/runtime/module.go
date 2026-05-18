package runtime

import (
	"log/slog"

	"github.com/arcgolabs/dix"
	"github.com/samber/mo"

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

type optionalProviderGroup struct {
	docker         dix.Conditional[*runtimedocker.Provider]
	podman         dix.Conditional[*runtimepodman.Provider]
	containerd     dix.Conditional[*runtimecontainerd.Provider]
	firecracker    dix.Conditional[*runtimefirecracker.Provider]
	systemd        dix.Conditional[*runtimesystemd.Provider]
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
			dix.ConditionalProvider3(optionalDockerProvider, dix.Eager()),
			dix.ConditionalProvider3(optionalPodmanProvider, dix.Eager()),
			dix.ConditionalProvider3(optionalContainerdProvider, dix.Eager()),
			dix.ConditionalProvider3(optionalFirecrackerProvider, dix.Eager()),
			dix.Provider2(runtimeprocess.NewProvider),
			dix.ConditionalProvider3(optionalSystemdProvider, dix.Eager()),
			dix.ConditionalProvider3(optionalWindowsServiceProvider, dix.Eager()),
			dix.Provider6(func(
				dockerProvider dix.Conditional[*runtimedocker.Provider],
				podmanProvider dix.Conditional[*runtimepodman.Provider],
				containerdProvider dix.Conditional[*runtimecontainerd.Provider],
				firecrackerProvider dix.Conditional[*runtimefirecracker.Provider],
				systemdProvider dix.Conditional[*runtimesystemd.Provider],
				windowsServiceProvider dix.Conditional[*runtimewindowsservice.Provider],
			) optionalProviderGroup {
				return optionalProviderGroup{
					docker:         dockerProvider,
					podman:         podmanProvider,
					containerd:     containerdProvider,
					firecracker:    firecrackerProvider,
					systemd:        systemdProvider,
					windowsService: windowsServiceProvider,
				}
			}, dix.Eager()),
			dix.Provider2(func(
				group optionalProviderGroup,
				processProvider *runtimeprocess.Provider,
			) providerList {
				return providerListFromConditionals(
					group.docker,
					group.podman,
					group.containerd,
					group.firecracker,
					processProvider,
					group.systemd,
					group.windowsService,
				)
			}, dix.Eager()),
			dix.Provider2(func(logger *slog.Logger, providers providerList) *Manager {
				return NewManager(logger, providers...)
			}, dix.Eager()),
		),
	)
}

func providerListFromConditionals(
	dockerProvider dix.Conditional[*runtimedocker.Provider],
	podmanProvider dix.Conditional[*runtimepodman.Provider],
	containerdProvider dix.Conditional[*runtimecontainerd.Provider],
	firecrackerProvider dix.Conditional[*runtimefirecracker.Provider],
	processProvider *runtimeprocess.Provider,
	systemdProvider dix.Conditional[*runtimesystemd.Provider],
	windowsServiceProvider dix.Conditional[*runtimewindowsservice.Provider],
) providerList {
	out := providerList{processProvider}
	if provider, ok := dockerProvider.Get(); ok {
		out = append(out, provider)
	}
	if provider, ok := podmanProvider.Get(); ok {
		out = append(out, provider)
	}
	if provider, ok := containerdProvider.Get(); ok {
		out = append(out, provider)
	}
	if provider, ok := firecrackerProvider.Get(); ok {
		out = append(out, provider)
	}
	if provider, ok := systemdProvider.Get(); ok {
		out = append(out, provider)
	}
	if provider, ok := windowsServiceProvider.Get(); ok {
		out = append(out, provider)
	}
	return out
}

func optionalDockerProvider(env Environment, logger *slog.Logger, dns *dnssvc.Service) dix.Conditional[*runtimedocker.Provider] {
	if !env.Docker {
		return mo.None[*runtimedocker.Provider]()
	}
	return mo.Some(runtimedocker.NewProvider(logger, dns))
}

func optionalPodmanProvider(env Environment, logger *slog.Logger, dns *dnssvc.Service) dix.Conditional[*runtimepodman.Provider] {
	if !env.Podman {
		return mo.None[*runtimepodman.Provider]()
	}
	return mo.Some(runtimepodman.NewProvider(logger, dns))
}

func optionalContainerdProvider(env Environment, logger *slog.Logger, dns *dnssvc.Service) dix.Conditional[*runtimecontainerd.Provider] {
	if !env.Containerd {
		return mo.None[*runtimecontainerd.Provider]()
	}
	return mo.Some(runtimecontainerd.NewProvider(logger, dns))
}

func optionalFirecrackerProvider(env Environment, logger *slog.Logger, dns *dnssvc.Service) dix.Conditional[*runtimefirecracker.Provider] {
	if !env.Firecracker {
		return mo.None[*runtimefirecracker.Provider]()
	}
	return mo.Some(runtimefirecracker.NewProvider(logger, dns))
}

func optionalSystemdProvider(env Environment, logger *slog.Logger, dns *dnssvc.Service) dix.Conditional[*runtimesystemd.Provider] {
	if !env.Systemd {
		return mo.None[*runtimesystemd.Provider]()
	}
	return mo.Some(runtimesystemd.NewProvider(logger, dns))
}

func optionalWindowsServiceProvider(env Environment, logger *slog.Logger, dns *dnssvc.Service) dix.Conditional[*runtimewindowsservice.Provider] {
	if !env.WindowsService {
		return mo.None[*runtimewindowsservice.Provider]()
	}
	return mo.Some(runtimewindowsservice.NewProvider(logger, dns))
}
