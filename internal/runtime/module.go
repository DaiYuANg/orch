package runtime

import (
	"log/slog"

	"github.com/arcgolabs/dix"

	"github.com/daiyuang/orch/internal/dnssvc"
	runtimecontainerd "github.com/daiyuang/orch/internal/runtime/containerd"
	runtimedocker "github.com/daiyuang/orch/internal/runtime/docker"
)

func Module() dix.Module {
	return dix.NewModule(
		"runtime",
		dix.Providers(
			dix.Provider2(func(logger *slog.Logger, dns *dnssvc.Service) *runtimedocker.Provider {
				return runtimedocker.NewProvider(logger, dns)
			}),
			dix.Provider2(func(logger *slog.Logger, dns *dnssvc.Service) *runtimecontainerd.Provider {
				return runtimecontainerd.NewProvider(logger, dns)
			}),
			dix.Provider3(func(logger *slog.Logger, dockerProvider *runtimedocker.Provider, containerdProvider *runtimecontainerd.Provider) *Manager {
				return NewManager(logger, dockerProvider, containerdProvider)
			}),
		),
	)
}
