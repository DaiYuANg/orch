package runtime

import (
	"log/slog"

	"github.com/arcgolabs/dix"

	runtimecontainerd "github.com/lyonbrown4d/orch/internal/runtime/containerd"
	runtimedocker "github.com/lyonbrown4d/orch/internal/runtime/docker"
	runtimefirecracker "github.com/lyonbrown4d/orch/internal/runtime/firecracker"
	runtimepodman "github.com/lyonbrown4d/orch/internal/runtime/podman"
	runtimeprocess "github.com/lyonbrown4d/orch/internal/runtime/process"
	runtimesystemd "github.com/lyonbrown4d/orch/internal/runtime/systemd"
	runtimewindowsservice "github.com/lyonbrown4d/orch/internal/runtime/windowsservice"
)

type providerList []Provider

func Module() dix.Module {
	return dix.NewModule(
		"runtime",
		dix.Providers(
			dix.Provider2(runtimedocker.NewProvider),
			dix.Provider2(runtimepodman.NewProvider),
			dix.Provider2(runtimecontainerd.NewProvider),
			dix.Provider2(runtimefirecracker.NewProvider),
			dix.Provider2(runtimeprocess.NewProvider),
			dix.Provider2(runtimesystemd.NewProvider),
			dix.Provider2(runtimewindowsservice.NewProvider),
			dix.Provider6(func(
				dockerProvider *runtimedocker.Provider,
				containerdProvider *runtimecontainerd.Provider,
				firecrackerProvider *runtimefirecracker.Provider,
				processProvider *runtimeprocess.Provider,
				systemdProvider *runtimesystemd.Provider,
				windowsServiceProvider *runtimewindowsservice.Provider,
			) providerList {
				return providerList{
					dockerProvider,
					containerdProvider,
					firecrackerProvider,
					processProvider,
					systemdProvider,
					windowsServiceProvider,
				}
			}, dix.Eager()),
			dix.Provider2(func(
				providers providerList,
				podmanProvider *runtimepodman.Provider,
			) providerList {
				out := make(providerList, 0, len(providers)+1)
				out = append(out, providers...)
				out = append(out, podmanProvider)
				return out
			}, dix.Eager()),
			dix.Provider2(func(logger *slog.Logger, providers providerList) *Manager {
				return NewManager(logger, providers...)
			}, dix.Eager()),
		),
	)
}
