package api

import (
	"context"

	"github.com/arcgolabs/httpx"

	orchruntime "github.com/lyonbrown4d/orch/internal/runtime"
)

// RuntimeProvidersEndpoint serves GET /api/v1/system/runtimes.
type RuntimeProvidersEndpoint struct {
	runtimes *orchruntime.Manager
}

func NewRuntimeProvidersEndpoint(runtimes *orchruntime.Manager) *RuntimeProvidersEndpoint {
	return &RuntimeProvidersEndpoint{runtimes: runtimes}
}

func (e *RuntimeProvidersEndpoint) EndpointSpec() httpx.EndpointSpec {
	return httpx.EndpointSpec{
		Prefix:      "/v1/system/runtimes",
		Description: "Runtime provider registration and host availability snapshot.",
		Tags:        httpx.Tags("system", "runtime"),
	}
}

func (e *RuntimeProvidersEndpoint) Register(r httpx.Registrar) {
	httpx.MustGroupGet(r.Scope(), "", e.Handle, OpenAPIMeta([]string{"system", "runtime"}, "runtime providers", "List runtime providers",
		"Returns runtime provider policy, detection, and registration state for the local server node."))
}

func (e *RuntimeProvidersEndpoint) Handle(context.Context, *EmptyInput) (*ListRuntimeProvidersOutput, error) {
	out := &ListRuntimeProvidersOutput{}
	if e == nil {
		out.Body.Items = runtimeProviderItems(nil)
		return out, nil
	}
	out.Body.Items = runtimeProviderItems(e.runtimes)
	return out, nil
}
