package api

import (
	"context"

	"github.com/arcgolabs/httpx"

	"github.com/daiyuang/orch/internal/services/task"
)

// AssignmentsEndpoint serves GET /api/v1/assignments.
type AssignmentsEndpoint struct {
	tasks *task.Service
}

func NewAssignmentsEndpoint(tasks *task.Service) *AssignmentsEndpoint {
	return &AssignmentsEndpoint{tasks: tasks}
}

func (e *AssignmentsEndpoint) EndpointSpec() httpx.EndpointSpec {
	return httpx.EndpointSpec{
		Prefix:      "/v1/assignments",
		Description: "Scheduler assignment state replicated through Raft.",
		Tags:        httpx.Tags("scheduler"),
	}
}

func (e *AssignmentsEndpoint) Register(r httpx.Registrar) {
	httpx.MustGroupGet(r.Scope(), "", e.handle, OpenAPIMeta([]string{"scheduler"}, "listWorkloadAssignments", "List scheduler workload assignments",
		"Sorted workload assignment records including app metadata, workload name, assigned node, runtime, image, status, and last error."))
}

func (e *AssignmentsEndpoint) handle(_ context.Context, _ *EmptyInput) (*ListAssignmentsOutput, error) {
	out := &ListAssignmentsOutput{}
	if e != nil && e.tasks != nil {
		out.Body.Items = e.tasks.ListWorkloadAssignments()
	}
	return out, nil
}
