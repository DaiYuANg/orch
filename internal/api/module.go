package api

import (
	"github.com/arcgolabs/dix"

	"github.com/daiyuang/orch/internal/config"
	"github.com/daiyuang/orch/internal/deploy/loader"
	"github.com/daiyuang/orch/internal/dixdiag"
	"github.com/daiyuang/orch/internal/dnssvc"
	"github.com/daiyuang/orch/internal/httpserver"
	"github.com/daiyuang/orch/internal/raftsvc"
	"github.com/daiyuang/orch/internal/services/registry"
	"github.com/daiyuang/orch/internal/services/task"
)

func Module() dix.Module {
	return dix.NewModule(
		"api",
		dix.Invokes(
			dix.RawInvoke(func(c *dix.Container) error {
				server, err := dix.ResolveAs[*httpserver.Server](c)
				if err != nil {
					return err
				}
				cfg, err := dix.ResolveAs[config.Config](c)
				if err != nil {
					return err
				}
				registrySvc, err := dix.ResolveAs[*registry.Service](c)
				if err != nil {
					return err
				}
				taskSvc, err := dix.ResolveAs[*task.Service](c)
				if err != nil {
					return err
				}
				loaderSvc, err := dix.ResolveAs[*loader.Loader](c)
				if err != nil {
					return err
				}
				dnsSvc, err := dix.ResolveAs[*dnssvc.Service](c)
				if err != nil {
					return err
				}
				raftSvc, err := dix.ResolveAs[*raftsvc.Service](c)
				if err != nil {
					return err
				}
				dixdiagSvc, err := dix.ResolveAs[*dixdiag.Service](c)
				if err != nil {
					return err
				}
				Register(server.Runtime(), cfg, registrySvc, taskSvc, loaderSvc, dnsSvc, raftSvc, dixdiagSvc)
				server.LogRegisteredRoutes()
				return nil
			}),
		),
	)
}
