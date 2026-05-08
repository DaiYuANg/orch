package api

import (
	"context"

	"github.com/arcgolabs/httpx"

	"github.com/daiyuang/orch/internal/services/task"
	"github.com/daiyuang/orch/internal/workerapi"
	"github.com/daiyuang/orch/internal/workloadmeta"
	"github.com/daiyuang/orch/pkg/oopsx"
)

// WorkerStopEndpoint serves POST /api/v1/worker/stop.
type WorkerStopEndpoint struct {
	tasks            *task.Service
	openAPIAuthApply bool
}

func NewWorkerStopEndpoint(tasks *task.Service, openAPIAuthApply bool) *WorkerStopEndpoint {
	return &WorkerStopEndpoint{tasks: tasks, openAPIAuthApply: openAPIAuthApply}
}

func (e *WorkerStopEndpoint) EndpointSpec() httpx.EndpointSpec {
	spec := httpx.EndpointSpec{
		Prefix:      "/v1/worker/stop",
		Description: "Stop a workload assigned to this worker node.",
		Tags:        httpx.Tags("worker"),
	}
	if e.openAPIAuthApply {
		spec.Security = httpx.SecurityRequirements(httpx.SecurityRequirement("bearerAuth"))
	}
	return spec
}

func (e *WorkerStopEndpoint) Register(r httpx.Registrar) {
	httpx.MustGroupPost(r.Scope(), "", e.handle, OpenAPIMeta([]string{"worker"}, "stopWorkerWorkload",
		"Stop assigned workload on this worker",
		"Internal worker endpoint used by scheduler stop dispatch. It stops the assigned workload on the local runtime and does not mutate Raft desired state."))
}

func (e *WorkerStopEndpoint) handle(ctx context.Context, in *workerapi.StopWorkloadInput) (*workerapi.StopWorkloadOutput, error) {
	if err := e.tasks.StopWorkerWorkload(ctx, in.Body.Metadata, in.Body.Workload, in.Body.Node); err != nil {
		return nil, oopsx.B("api", "worker").Wrapf(err, "stop worker workload")
	}
	out := &workerapi.StopWorkloadOutput{}
	out.Body.Accepted = true
	out.Body.Node = in.Body.Node
	out.Body.Status = workloadmeta.AssignmentStatusStopped
	out.Body.Workload = in.Body.Workload.Name
	return out, nil
}
