package merics

import "github.com/danielgtaylor/huma/v2"

func (e *Endpoint) Register(openapi huma.API) {
	tag := huma.OperationTags("metrics")
	huma.Get(openapi, "/metrics/ping", e.PingHandler, tag)
}

func NewMetricsEndpoint() *Endpoint {
	return &Endpoint{}
}
