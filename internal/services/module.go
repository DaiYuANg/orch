package services

import (
	"github.com/arcgolabs/dix"

	"github.com/daiyuang/orch/internal/services/registry"
	"github.com/daiyuang/orch/internal/services/task"
)

func Module() dix.Module {
	return dix.NewModule(
		"services",
		dix.Providers(
			dix.Provider1(registry.NewService),
			dix.Provider4(task.NewService),
		),
	)
}
