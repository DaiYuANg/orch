package runtime

import (
	"log/slog"

	"github.com/arcgolabs/dix"

	runtimecontainerd "github.com/daiyuang/orch/internal/runtime/containerd"
	runtimedocker "github.com/daiyuang/orch/internal/runtime/docker"
	runtimefirecracker "github.com/daiyuang/orch/internal/runtime/firecracker"
	runtimeprocess "github.com/daiyuang/orch/internal/runtime/process"
	runtimesystemd "github.com/daiyuang/orch/internal/runtime/systemd"
	runtimewindowsservice "github.com/daiyuang/orch/internal/runtime/windowsservice"
)

type providerList []Provider

func Module() dix.Module {
	return dix.NewModule(
		"runtime",
		dix.Providers(
			dix.Provider2(runtimedocker.NewProvider),
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
				return providerList{dockerProvider, containerdProvider, firecrackerProvider, processProvider, systemdProvider, windowsServiceProvider}
			}, dix.Eager()),
			dix.Provider2(func(logger *slog.Logger, providers providerList) *Manager {
				return NewManager(logger, providers...)
			}, dix.Eager()),
		),
	)
}
