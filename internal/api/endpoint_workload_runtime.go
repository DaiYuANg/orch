package api

import (
	"context"
	"strings"

	"github.com/arcgolabs/httpx"

	deployv1 "github.com/lyonbrown4d/orch/internal/deploy/v1alpha1"
	"github.com/lyonbrown4d/orch/internal/runtime/runtimeinfo"
	"github.com/lyonbrown4d/orch/internal/services/task"
	"github.com/lyonbrown4d/orch/pkg/oopsx"
)

type WorkloadRuntimeEndpoint struct {
	tasks *task.Service
}

func NewWorkloadRuntimeEndpoint(tasks *task.Service) *WorkloadRuntimeEndpoint {
	return &WorkloadRuntimeEndpoint{tasks: tasks}
}

func (e *WorkloadRuntimeEndpoint) EndpointSpec() httpx.EndpointSpec {
	return httpx.EndpointSpec{
		Prefix:      "/v1/workloads",
		Description: "Runtime-local workload inspection.",
		Tags:        httpx.Tags("workloads"),
	}
}

func (e *WorkloadRuntimeEndpoint) Register(r httpx.Registrar) {
	httpx.MustGroupGet(r.Scope(), "/{namespace}/{app}/{workload}/status", e.status, OpenAPIMeta([]string{"workloads"}, "getWorkloadRuntimeStatus",
		"Get workload runtime status",
		"Returns runtime-local status for a workload in a desired app."))
	httpx.MustGroupGet(r.Scope(), "/{namespace}/{app}/{workload}/logs", e.logs, OpenAPIMeta([]string{"workloads"}, "getWorkloadLogs",
		"Get workload logs",
		"Returns recent runtime logs for a workload when the provider supports logs."))
}

func (e *WorkloadRuntimeEndpoint) status(ctx context.Context, in *WorkloadRuntimeInput) (*WorkloadRuntimeStatusOutput, error) {
	if e == nil || e.tasks == nil {
		return nil, oopsx.B("api").Errorf("task service unavailable")
	}
	meta, workloadName, err := workloadRuntimePath(in.Namespace, in.App, in.Workload)
	if err != nil {
		return nil, err
	}
	status, err := e.tasks.WorkloadRuntimeStatus(ctx, meta, workloadName)
	if err != nil {
		return nil, oopsx.B("api").Wrapf(err, "get workload runtime status")
	}
	out := &WorkloadRuntimeStatusOutput{}
	out.Body = status
	return out, nil
}

func (e *WorkloadRuntimeEndpoint) logs(ctx context.Context, in *WorkloadLogsInput) (*WorkloadLogsOutput, error) {
	if e == nil || e.tasks == nil {
		return nil, oopsx.B("api").Errorf("task service unavailable")
	}
	meta, workloadName, err := workloadRuntimePath(in.Namespace, in.App, in.Workload)
	if err != nil {
		return nil, err
	}
	logs, err := e.tasks.WorkloadRuntimeLogs(ctx, meta, workloadName, runtimeinfo.LogOptions{Tail: in.Tail})
	if err != nil {
		return nil, oopsx.B("api").Wrapf(err, "get workload logs")
	}
	out := &WorkloadLogsOutput{}
	out.Body = logs
	return out, nil
}

func workloadRuntimePath(namespace, app, workload string) (deployv1.Metadata, string, error) {
	meta := deployv1.Metadata{
		Namespace: strings.TrimSpace(namespace),
		Name:      strings.TrimSpace(app),
	}
	workloadName := strings.TrimSpace(workload)
	if meta.Name == "" {
		return meta, "", oopsx.B("api").Errorf("app name is required")
	}
	if workloadName == "" {
		return meta, "", oopsx.B("api").Errorf("workload name is required")
	}
	return meta, workloadName, nil
}
