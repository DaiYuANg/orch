package api

import (
	"context"
	"time"

	"github.com/arcgolabs/httpx"
)

// HealthEndpoint serves GET /api/health (Prefix /health under server base /api).
type HealthEndpoint struct{}

func NewHealthEndpoint() *HealthEndpoint {
	return &HealthEndpoint{}
}

func (e *HealthEndpoint) EndpointSpec() httpx.EndpointSpec {
	return httpx.EndpointSpec{
		Prefix:      "/health",
		Description: "Liveness and connectivity.",
		Tags:        httpx.Tags("system"),
	}
}

func (e *HealthEndpoint) Register(r httpx.Registrar) {
	httpx.MustGroupGet(r.Scope(), "", e.handle, OpenAPIMeta([]string{"system"}, "health", "Liveness and basic connectivity check"))
}

func (e *HealthEndpoint) handle(_ context.Context, _ *EmptyInput) (*HealthOutput, error) {
	out := &HealthOutput{}
	out.Body.Status = "ok"
	out.Body.Timestamp = time.Now().UTC().Format(time.RFC3339)
	return out, nil
}
