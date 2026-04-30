package api

import (
	"github.com/arcgolabs/dix"

	"github.com/daiyuang/orch/internal/config"
	"github.com/daiyuang/orch/internal/deploy/loader"
	"github.com/daiyuang/orch/internal/httpserver"
	"github.com/daiyuang/orch/internal/services/registry"
	"github.com/daiyuang/orch/internal/services/task"
)

func Module() dix.Module {
	return dix.NewModule(
		"api",
		dix.Invokes(
			dix.Invoke5(func(server *httpserver.Server, cfg config.Config, registrySvc *registry.Service, taskSvc *task.Service, loaderSvc *loader.Loader) {
				Register(server.Runtime(), cfg, registrySvc, taskSvc, loaderSvc)
				server.LogRegisteredRoutes()
			}),
		),
	)
}
