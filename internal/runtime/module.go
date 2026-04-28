package runtime

import (
	"log/slog"

	"github.com/arcgolabs/dix"

	runtimecontainerd "github.com/daiyuang/orch/internal/runtime/containerd"
	runtimedocker "github.com/daiyuang/orch/internal/runtime/docker"
)

func Module() dix.Module {
	return dix.NewModule(
		"runtime",
		dix.Providers(
			dix.Provider1(func(logger *slog.Logger) *runtimedocker.Provider {
				return runtimedocker.NewProvider(logger)
			}),
			dix.Provider1(func(logger *slog.Logger) *runtimecontainerd.Provider {
				return runtimecontainerd.NewProvider(logger)
			}),
			dix.Provider3(func(logger *slog.Logger, dockerProvider *runtimedocker.Provider, containerdProvider *runtimecontainerd.Provider) *Manager {
				return NewManager(logger, dockerProvider, containerdProvider)
			}),
		),
	)
}

