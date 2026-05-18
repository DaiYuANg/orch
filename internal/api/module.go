package api

import (
	"github.com/arcgolabs/dix"

	"github.com/lyonbrown4d/orch/internal/httpserver"
)

func Module() dix.Module {
	return dix.NewModule(
		"api",
		dix.Providers(
			dix.Provider1(newOpenAPIAuthApply),
			dix.Provider2(newLeaderForwarderProvider),
			dix.Provider4(newSystemEndpoints),
			dix.Provider2(newWorkloadEndpoints),
			dix.Provider4(newRaftEndpoints),
			dix.Provider4(newDeployEndpoints),
			dix.Provider2(newWorkerEndpoints),
			dix.Provider5(newRouteEndpoints),
		),
		dix.Invokes(
			dix.Invoke2(func(server *httpserver.Server, endpoints RouteEndpoints) {
				RegisterEndpoints(server.Runtime(), endpoints)
				server.LogRegisteredRoutes()
			}),
		),
	)
}
