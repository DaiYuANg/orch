package api

import (
	"github.com/arcgolabs/httpx"

	"github.com/daiyuang/orch/internal/config"
	"github.com/daiyuang/orch/internal/deploy/loader"
	"github.com/daiyuang/orch/internal/dnssvc"
	"github.com/daiyuang/orch/internal/raftsvc"
	"github.com/daiyuang/orch/internal/services/registry"
	"github.com/daiyuang/orch/internal/services/task"
)

// Register wires all HTTP [httpx.Endpoint] modules in one place. Each module owns its Prefix and handlers;
// route paths use "" to bind to that prefix root (see per-type EndpointSpec).
func Register(rt httpx.ServerRuntime, cfg config.Config, registrySvc *registry.Service, taskSvc *task.Service, loaderSvc *loader.Loader, dnsSvc *dnssvc.Service, raftSvc *raftsvc.Service) {
	rt.RegisterOnly(
		NewHealthEndpoint(),
		NewHostinfoEndpoint(),
		NewAppsEndpoint(taskSvc),
		NewWorkloadsEndpoint(registrySvc),
		NewAssignmentsEndpoint(taskSvc),
		NewOrchVPNBootstrapEndpoint(cfg, dnsSvc),
		NewRaftStatusEndpoint(raftSvc, cfg.Auth.Enabled),
		NewRaftMembersEndpoint(raftSvc, cfg.Auth.Enabled),
		NewDeployEndpoint(taskSvc, cfg.Auth.Enabled),
		NewStartDeployEndpoint(taskSvc, cfg.Auth.Enabled),
		NewStopDeployEndpoint(taskSvc, cfg.Auth.Enabled),
		NewRestartDeployEndpoint(taskSvc, cfg.Auth.Enabled),
		NewDeleteDeployEndpoint(taskSvc, cfg.Auth.Enabled),
		NewDeploySourceEndpoint(loaderSvc, taskSvc, cfg.Auth.Enabled),
		NewWorkerDeployEndpoint(taskSvc, cfg.Auth.Enabled),
		NewWorkerStopEndpoint(taskSvc, cfg.Auth.Enabled),
	)
}
