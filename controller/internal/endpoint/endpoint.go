package endpoint

import (
	"github.com/danielgtaylor/huma/v2"
	"go.uber.org/fx"
)

type Endpoint interface {
	Register(openapi huma.API)
}

func annotation[T any](endpoint T) interface{} {
	return fx.Annotate(
		endpoint,
		fx.ResultTags(`group:"endpoint"`),
		fx.As(new(Endpoint)),
	)
}
