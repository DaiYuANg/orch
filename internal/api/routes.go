package api

import (
	"github.com/arcgolabs/httpx"

	"github.com/daiyuang/orch/internal/config"
	"github.com/daiyuang/orch/internal/services/registry"
	"github.com/daiyuang/orch/internal/services/task"
)

// Register wires all HTTP [httpx.Endpoint] modules in one place. Each module owns its Prefix and handlers;
// route paths use "" to bind to that prefix root (see per-type EndpointSpec).
func Register(rt httpx.ServerRuntime, cfg config.Config, registrySvc *registry.Service, taskSvc *task.Service) {
	rt.RegisterOnly(
		NewHealthEndpoint(),
		NewHostinfoEndpoint(),
		NewWorkloadsEndpoint(registrySvc),
		NewDeployEndpoint(taskSvc, cfg.Auth.Enabled),
	)
}
