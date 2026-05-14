package api

import (
	"github.com/arcgolabs/httpx"

	"github.com/lyonbrown4d/orch/internal/config"
	"github.com/lyonbrown4d/orch/internal/deploy/loader"
	"github.com/lyonbrown4d/orch/internal/dixdiag"
	"github.com/lyonbrown4d/orch/internal/dnssvc"
	"github.com/lyonbrown4d/orch/internal/raftsvc"
	"github.com/lyonbrown4d/orch/internal/services/registry"
	"github.com/lyonbrown4d/orch/internal/services/task"
)

// Register wires all HTTP [httpx.Endpoint] modules in one place. Each module owns its Prefix and handlers;
// route paths use "" to bind to that prefix root (see per-type EndpointSpec).
func Register(rt httpx.ServerRuntime, cfg config.Config, registrySvc *registry.Service, taskSvc *task.Service, loaderSvc *loader.Loader, dnsSvc *dnssvc.Service, raftSvc *raftsvc.Service, dixdiagSvcs ...*dixdiag.Service) {
	var dixdiagSvc *dixdiag.Service
	if len(dixdiagSvcs) > 0 {
		dixdiagSvc = dixdiagSvcs[0]
	}
	leader := NewLeaderForwarder(cfg, raftSvc)
	rt.RegisterOnly(
		NewHealthEndpoint(dixdiagSvc),
		NewReadyEndpoint(cfg, raftSvc, dixdiagSvc),
		NewDiagnosticsEndpoint(dixdiagSvc),
		NewHostinfoEndpoint(),
		NewAppsEndpoint(taskSvc),
		NewWorkloadsEndpoint(registrySvc),
		NewWorkloadRuntimeEndpoint(taskSvc),
		NewAssignmentsEndpoint(taskSvc),
		NewOrchVPNBootstrapEndpoint(cfg, dnsSvc),
		NewRaftStatusEndpoint(cfg, raftSvc, cfg.Auth.Enabled),
		NewRaftMembersEndpoint(raftSvc, leader, cfg.Auth.Enabled),
		NewDeployEndpoint(taskSvc, leader, cfg.Auth.Enabled),
		NewStartDeployEndpoint(taskSvc, leader, cfg.Auth.Enabled),
		NewStopDeployEndpoint(taskSvc, leader, cfg.Auth.Enabled),
		NewRestartDeployEndpoint(taskSvc, leader, cfg.Auth.Enabled),
		NewMigrateDeployEndpoint(taskSvc, leader, cfg.Auth.Enabled),
		NewFailoverDeployEndpoint(taskSvc, leader, cfg.Auth.Enabled),
		NewRebalanceDeployEndpoint(taskSvc, leader, cfg.Auth.Enabled),
		NewDeleteDeployEndpoint(taskSvc, leader, cfg.Auth.Enabled),
		NewDeploySourceEndpoint(loaderSvc, taskSvc, leader, cfg.Auth.Enabled),
		NewWorkerDeployEndpoint(taskSvc, cfg.Auth.Enabled),
		NewWorkerStopEndpoint(taskSvc, cfg.Auth.Enabled),
		NewWorkerStatusEndpoint(taskSvc, cfg.Auth.Enabled),
		NewWorkerLogsEndpoint(taskSvc, cfg.Auth.Enabled),
	)
}
