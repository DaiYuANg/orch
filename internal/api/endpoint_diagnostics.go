package api

import (
	"context"

	"github.com/arcgolabs/httpx"

	"github.com/daiyuang/orch/internal/dixdiag"
)

type DiagnosticsEndpoint struct {
	diag *dixdiag.Service
}

func NewDiagnosticsEndpoint(diag *dixdiag.Service) *DiagnosticsEndpoint {
	return &DiagnosticsEndpoint{diag: diag}
}

func (e *DiagnosticsEndpoint) EndpointSpec() httpx.EndpointSpec {
	return httpx.EndpointSpec{
		Prefix:      "/v1/diagnostics",
		Description: "Dix runtime diagnostics.",
		Tags:        httpx.Tags("system"),
	}
}

func (e *DiagnosticsEndpoint) Register(r httpx.Registrar) {
	httpx.MustGroupGet(r.Scope(), "", e.handle, OpenAPIMeta([]string{"system"}, "diagnostics", "Dix runtime diagnostics",
		"Returns lifecycle, recent event, and dependency graph diagnostics for this control-plane process."))
}

func (e *DiagnosticsEndpoint) handle(_ context.Context, _ *EmptyInput) (*DiagnosticsOutput, error) {
	out := &DiagnosticsOutput{}
	if e != nil && e.diag != nil {
		out.Body = e.diag.Snapshot()
	}
	return out, nil
}
