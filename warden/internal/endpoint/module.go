package endpoint

import (
	"github.com/DaiYuANg/warden/internal/endpoint/task"
	"github.com/danielgtaylor/huma/v2"
	"github.com/samber/lo"
	"go.uber.org/fx"
)

var Module = fx.Module("endpoint",
	fx.Provide(
		annotation(task.NewTaskEndpoint),
	),
	fx.Invoke(registerEndpoint),
)

type RegisterEndpointParameter struct {
	fx.In
	Endpoint []Endpoint `group:"endpoint"`
	Openapi  huma.API
}

func registerEndpoint(parameters RegisterEndpointParameter) {
	endpoints, openapi := parameters.Endpoint, parameters.Openapi
	lo.ForEach(endpoints, func(item Endpoint, _ int) {
		item.Register(openapi)
	})
}
