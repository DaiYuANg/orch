package api

import (
	"github.com/arcgolabs/dix"

	"github.com/daiyuang/orch/internal/httpserver"
	"github.com/daiyuang/orch/internal/services/registry"
	"github.com/daiyuang/orch/internal/services/task"
)

func Module() dix.Module {
	return dix.NewModule(
		"api",
		dix.Invokes(
			dix.Invoke3(func(server *httpserver.Server, registrySvc *registry.Service, taskSvc *task.Service) {
				Register(server.Runtime(), registrySvc, taskSvc)
			}),
		),
	)
}

