package api

import (
	"context"

	"github.com/arcgolabs/httpx"

	"github.com/lyonbrown4d/orch/internal/runtime/runtimeinfo"
	"github.com/lyonbrown4d/orch/internal/services/task"
	"github.com/lyonbrown4d/orch/internal/workerapi"
	"github.com/lyonbrown4d/orch/pkg/oopsx"
)

// WorkerLogsEndpoint serves POST /api/v1/worker/logs.
type WorkerLogsEndpoint struct {
	tasks            *task.Service
	openAPIAuthApply bool
}

func NewWorkerLogsEndpoint(tasks *task.Service, openAPIAuthApply bool) *WorkerLogsEndpoint {
	return &WorkerLogsEndpoint{tasks: tasks, openAPIAuthApply: openAPIAuthApply}
}

func (e *WorkerLogsEndpoint) EndpointSpec() httpx.EndpointSpec {
	spec := httpx.EndpointSpec{
		Prefix:      "/v1/worker/logs",
		Description: "Read logs for a workload assigned to this worker node.",
		Tags:        httpx.Tags("worker"),
	}
	if e.openAPIAuthApply {
		spec.Security = httpx.SecurityRequirements(httpx.SecurityRequirement("bearerAuth"))
	}
	return spec
}

func (e *WorkerLogsEndpoint) Register(r httpx.Registrar) {
	httpx.MustGroupPost(r.Scope(), "", e.handle, OpenAPIMeta([]string{"worker"}, "logsWorkerWorkload",
		"Read assigned workload logs on this worker",
		"Internal worker endpoint used by scheduler runtime inspection. It reads local runtime logs and does not mutate Raft desired state."))
}

func (e *WorkerLogsEndpoint) handle(ctx context.Context, in *workerapi.WorkloadLogsInput) (*workerapi.WorkloadLogsOutput, error) {
	logs, err := e.tasks.WorkerWorkloadRuntimeLogs(ctx, in.Body.Metadata, in.Body.Workload, in.Body.Node, runtimeinfo.LogOptions{Tail: in.Body.Tail})
	if err != nil {
		return nil, oopsx.B("api", "worker").Wrapf(err, "worker workload logs")
	}
	out := &workerapi.WorkloadLogsOutput{}
	out.Body = logs
	return out, nil
}
