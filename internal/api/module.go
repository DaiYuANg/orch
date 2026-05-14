package api

import (
	"github.com/arcgolabs/dix"

	"github.com/lyonbrown4d/orch/internal/config"
	"github.com/lyonbrown4d/orch/internal/deploy/loader"
	"github.com/lyonbrown4d/orch/internal/dixdiag"
	"github.com/lyonbrown4d/orch/internal/dnssvc"
	"github.com/lyonbrown4d/orch/internal/httpserver"
	"github.com/lyonbrown4d/orch/internal/raftsvc"
	"github.com/lyonbrown4d/orch/internal/services/registry"
	"github.com/lyonbrown4d/orch/internal/services/task"
)

func Module() dix.Module {
	return dix.NewModule(
		"api",
		dix.Invokes(
			dix.RawInvoke(func(c *dix.Container) error {
				deps, err := resolveModuleDeps(c)
				if err != nil {
					return err
				}
				Register(
					deps.server.Runtime(),
					deps.cfg,
					deps.registry,
					deps.task,
					deps.loader,
					deps.dns,
					deps.raft,
					deps.dixdiag,
				)
				deps.server.LogRegisteredRoutes()
				return nil
			}),
		),
	)
}

type moduleDeps struct {
	server   *httpserver.Server
	cfg      config.Config
	registry *registry.Service
	task     *task.Service
	loader   *loader.Loader
	dns      *dnssvc.Service
	raft     *raftsvc.Service
	dixdiag  *dixdiag.Service
}

func resolveModuleDeps(c *dix.Container) (moduleDeps, error) {
	core, err := resolveCoreModuleDeps(c)
	if err != nil {
		return moduleDeps{}, err
	}
	control, err := resolveControlModuleDeps(c)
	if err != nil {
		return moduleDeps{}, err
	}
	return moduleDeps{
		server:   core.server,
		cfg:      core.cfg,
		registry: core.registry,
		task:     core.task,
		loader:   control.loader,
		dns:      control.dns,
		raft:     control.raft,
		dixdiag:  control.dixdiag,
	}, nil
}

type coreModuleDeps struct {
	server   *httpserver.Server
	cfg      config.Config
	registry *registry.Service
	task     *task.Service
}

func resolveCoreModuleDeps(c *dix.Container) (coreModuleDeps, error) {
	server, err := dix.ResolveAs[*httpserver.Server](c)
	if err != nil {
		return coreModuleDeps{}, err
	}
	cfg, err := dix.ResolveAs[config.Config](c)
	if err != nil {
		return coreModuleDeps{}, err
	}
	registrySvc, err := dix.ResolveAs[*registry.Service](c)
	if err != nil {
		return coreModuleDeps{}, err
	}
	taskSvc, err := dix.ResolveAs[*task.Service](c)
	if err != nil {
		return coreModuleDeps{}, err
	}
	return coreModuleDeps{server: server, cfg: cfg, registry: registrySvc, task: taskSvc}, nil
}

type controlModuleDeps struct {
	loader  *loader.Loader
	dns     *dnssvc.Service
	raft    *raftsvc.Service
	dixdiag *dixdiag.Service
}

func resolveControlModuleDeps(c *dix.Container) (controlModuleDeps, error) {
	loaderSvc, err := dix.ResolveAs[*loader.Loader](c)
	if err != nil {
		return controlModuleDeps{}, err
	}
	dnsSvc, err := dix.ResolveAs[*dnssvc.Service](c)
	if err != nil {
		return controlModuleDeps{}, err
	}
	raftSvc, err := dix.ResolveAs[*raftsvc.Service](c)
	if err != nil {
		return controlModuleDeps{}, err
	}
	dixdiagSvc, err := dix.ResolveAs[*dixdiag.Service](c)
	if err != nil {
		return controlModuleDeps{}, err
	}
	return controlModuleDeps{loader: loaderSvc, dns: dnsSvc, raft: raftSvc, dixdiag: dixdiagSvc}, nil
}
