package runtime

import (
	"log/slog"

	"github.com/arcgolabs/dix"

	runtimecontainerd "github.com/daiyuang/orch/internal/runtime/containerd"
	runtimedocker "github.com/daiyuang/orch/internal/runtime/docker"
	runtimeprocess "github.com/daiyuang/orch/internal/runtime/process"
	runtimesystemd "github.com/daiyuang/orch/internal/runtime/systemd"
	runtimewindowsservice "github.com/daiyuang/orch/internal/runtime/windowsservice"
)

func Module() dix.Module {
	return dix.NewModule(
		"runtime",
		dix.Providers(
			dix.Provider2(runtimedocker.NewProvider),
			dix.Provider2(runtimecontainerd.NewProvider),
			dix.Provider2(runtimeprocess.NewProvider),
			dix.Provider2(runtimesystemd.NewProvider),
			dix.Provider2(runtimewindowsservice.NewProvider),
			dix.Provider6(func(
				logger *slog.Logger,
				dockerProvider *runtimedocker.Provider,
				containerdProvider *runtimecontainerd.Provider,
				processProvider *runtimeprocess.Provider,
				systemdProvider *runtimesystemd.Provider,
				windowsServiceProvider *runtimewindowsservice.Provider,
			) *Manager {
				return NewManager(logger, dockerProvider, containerdProvider, processProvider, systemdProvider, windowsServiceProvider)
			}),
		),
	)
}
