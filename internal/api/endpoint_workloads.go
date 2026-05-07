package api

import (
	"context"

	"github.com/arcgolabs/collectionx/list"
	"github.com/arcgolabs/httpx"

	"github.com/daiyuang/orch/internal/services/registry"
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
		"Sorted workload records (including name, runtime, artifact, status) from this node registry."))
}

func (e *WorkloadsEndpoint) handle(_ context.Context, _ *EmptyInput) (*ListWorkloadsOutput, error) {
	out := &ListWorkloadsOutput{}
	out.Body.Items = list.MapList(e.registry.List(), func(_ int, record registry.WorkloadRecord) WorkloadItem {
		return WorkloadItem{
			Name:      record.Name,
			Node:      record.Node,
			Runtime:   record.Runtime,
			Artifact:  record.Artifact,
			Status:    record.Status,
			UpdatedAt: record.UpdatedAt,
		}
	})
	return out, nil
}
