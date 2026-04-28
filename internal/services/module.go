package services

import (
	"github.com/arcgolabs/dix"

	"github.com/daiyuang/orch/internal/nodecapacity"
	"github.com/daiyuang/orch/internal/placement"
	"github.com/daiyuang/orch/internal/raftsvc"
	"github.com/daiyuang/orch/internal/services/registry"
	"github.com/daiyuang/orch/internal/services/task"
)

func Module() dix.Module {
	return dix.NewModule(
		"services",
		dix.Providers(
			dix.Provider0(placement.NewEngine),
			dix.Provider1(func(rs *raftsvc.Service) *nodecapacity.Catalog {
				return nodecapacity.NewCatalog(raftsvc.NewRaftCapacityStore(rs))
			}),
			dix.Provider3(task.NewBundle),
			dix.Provider1(registry.NewService),
			dix.Provider6(task.NewService),
		),
	)
}
