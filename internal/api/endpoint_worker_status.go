package api

import (
	"context"

	"github.com/arcgolabs/httpx"

	"github.com/lyonbrown4d/orch/internal/services/task"
	"github.com/lyonbrown4d/orch/internal/workerapi"
	"github.com/lyonbrown4d/orch/pkg/oopsx"
)

// WorkerStatusEndpoint serves POST /api/v1/worker/status.
type WorkerStatusEndpoint struct {
	tasks            *task.Service
	openAPIAuthApply bool
}

func NewWorkerStatusEndpoint(tasks *task.Service, openAPIAuthApply bool) *WorkerStatusEndpoint {
	return &WorkerStatusEndpoint{tasks: tasks, openAPIAuthApply: openAPIAuthApply}
}

func (e *WorkerStatusEndpoint) EndpointSpec() httpx.EndpointSpec {
	spec := httpx.EndpointSpec{
		Prefix:      "/v1/worker/status",
		Description: "Inspect a workload assigned to this worker node.",
		Tags:        httpx.Tags("worker"),
	}
	if e.openAPIAuthApply {
		spec.Security = httpx.SecurityRequirements(httpx.SecurityRequirement("bearerAuth"))
	}
	return spec
}

func (e *WorkerStatusEndpoint) Register(r httpx.Registrar) {
	httpx.MustGroupPost(r.Scope(), "", e.handle, OpenAPIMeta([]string{"worker"}, "statusWorkerWorkload",
		"Inspect assigned workload on this worker",
		"Internal worker endpoint used by scheduler runtime inspection. It reads local runtime status and does not mutate Raft desired state."))
}

func (e *WorkerStatusEndpoint) handle(ctx context.Context, in *workerapi.WorkloadStatusInput) (*workerapi.WorkloadStatusOutput, error) {
	status, err := e.tasks.WorkerWorkloadRuntimeStatus(ctx, in.Body.Metadata, in.Body.Workload, in.Body.Node)
	if err != nil {
		return nil, oopsx.B("api", "worker").Wrapf(err, "worker workload status")
	}
	out := &workerapi.WorkloadStatusOutput{}
	out.Body = status
	return out, nil
}
