package api

import (
	"context"
	"time"

	"github.com/arcgolabs/collectionx/list"
	"github.com/arcgolabs/httpx"

	"github.com/daiyuang/orch/internal/dixdiag"
)

// HealthEndpoint serves GET /api/health (Prefix /health under server base /api).
type HealthEndpoint struct {
	diag *dixdiag.Service
}

func NewHealthEndpoint(diag *dixdiag.Service) *HealthEndpoint {
	return &HealthEndpoint{diag: diag}
}

func (e *HealthEndpoint) EndpointSpec() httpx.EndpointSpec {
	return httpx.EndpointSpec{
		Prefix:      "/health",
		Description: "Liveness and connectivity.",
		Tags:        httpx.Tags("system"),
	}
}

func (e *HealthEndpoint) Register(r httpx.Registrar) {
	httpx.MustGroupGet(r.Scope(), "", e.handle, OpenAPIMeta([]string{"system"}, "health", "Liveness and basic connectivity check",
		"Public endpoint. Returns dix health checks, aggregate health status, and a UTC RFC3339 timestamp."))
}

func (e *HealthEndpoint) handle(ctx context.Context, _ *EmptyInput) (*HealthOutput, error) {
	healthy := true
	checks := list.NewList[ReadyCheckItem]()
	if e != nil && e.diag != nil {
		report := e.diag.CheckHealth(ctx)
		healthy = report.Healthy()
		checks = dixHealthItems("", report)
	}

	out := &HealthOutput{}
	out.Body.Healthy = healthy
	out.Body.Status = healthStatus(healthy)
	out.Body.Timestamp = time.Now().UTC().Format(time.RFC3339)
	out.Body.Checks = checks
	return out, nil
}

func healthStatus(healthy bool) string {
	if healthy {
		return "ok"
	}
	return "unhealthy"
}
