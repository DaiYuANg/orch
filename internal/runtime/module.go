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
			dix.Provider2(runtimedocker.NewProvider),
			dix.Provider2(runtimecontainerd.NewProvider),
			dix.Provider3(func(logger *slog.Logger, dockerProvider *runtimedocker.Provider, containerdProvider *runtimecontainerd.Provider) *Manager {
				return NewManager(logger, dockerProvider, containerdProvider)
			}),
		),
	)
}
