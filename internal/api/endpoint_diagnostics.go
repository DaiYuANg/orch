package api

import (
	"context"

	"github.com/arcgolabs/httpx"

	"github.com/lyonbrown4d/orch/internal/dixdiag"
	orchruntime "github.com/lyonbrown4d/orch/internal/runtime"
)

type DiagnosticsEndpoint struct {
	diag     *dixdiag.Service
	runtimes *orchruntime.Manager
}

func NewDiagnosticsEndpoint(diag *dixdiag.Service, runtimes *orchruntime.Manager) *DiagnosticsEndpoint {
	return &DiagnosticsEndpoint{diag: diag, runtimes: runtimes}
}

func (e *DiagnosticsEndpoint) EndpointSpec() httpx.EndpointSpec {
	return httpx.EndpointSpec{
		Prefix:      "/v1/diagnostics",
		Description: "Dix runtime diagnostics.",
		Tags:        httpx.Tags("system"),
	}
}

func (e *DiagnosticsEndpoint) Register(r httpx.Registrar) {
	httpx.MustGroupGet(r.Scope(), "", e.Handle, OpenAPIMeta([]string{"system"}, "diagnostics", "Dix runtime diagnostics",
		"Returns lifecycle, recent event, and dependency graph diagnostics for this control-plane process."))
}

func (e *DiagnosticsEndpoint) Handle(_ context.Context, _ *EmptyInput) (*DiagnosticsOutput, error) {
	out := &DiagnosticsOutput{}
	if e != nil && e.diag != nil {
		out.Body.Snapshot = e.diag.Snapshot()
	}
	if e != nil {
		out.Body.Runtime = runtimeDiagnostics(e.runtimes)
	}
	return out, nil
}
