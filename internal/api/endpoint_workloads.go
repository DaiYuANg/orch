package api

import (
	"context"

	"github.com/arcgolabs/httpx"
	"github.com/arcgolabs/mapper"

	"github.com/daiyuang/orch/internal/services/registry"
	"github.com/daiyuang/orch/pkg/oopsx"
)

// WorkloadsEndpoint serves GET /api/v1/workloads.
type WorkloadsEndpoint struct {
	registry *registry.Service
}

func NewWorkloadsEndpoint(reg *registry.Service) *WorkloadsEndpoint {
	return &WorkloadsEndpoint{registry: reg}
}

func (e *WorkloadsEndpoint) EndpointSpec() httpx.EndpointSpec {
	return httpx.EndpointSpec{
		Prefix:      "/v1/workloads",
		Description: "Workload registry view for this node.",
		Tags:        httpx.Tags("registry"),
	}
}

func (e *WorkloadsEndpoint) Register(r httpx.Registrar) {
	httpx.MustGroupGet(r.Scope(), "", e.handle, OpenAPIMeta([]string{"registry"}, "listWorkloads", "List workloads known to this node",
		"Sorted workload records (including name, runtime, image, status) from this node registry."))
}

func (e *WorkloadsEndpoint) handle(_ context.Context, _ *EmptyInput) (*ListWorkloadsOutput, error) {
	out := &ListWorkloadsOutput{}
	items, err := mapper.MapSlice[WorkloadItem](e.registry.List())
	if err != nil {
		return nil, oopsx.B("api").Wrapf(err, "map workload records")
	}
	out.Body.Items = items
	return out, nil
}
