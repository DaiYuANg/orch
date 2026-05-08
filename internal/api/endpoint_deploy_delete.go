package api

import (
	"context"

	"github.com/arcgolabs/httpx"

	deployv1 "github.com/daiyuang/orch/internal/deploy/v1alpha1"
	"github.com/daiyuang/orch/internal/services/task"
	"github.com/daiyuang/orch/internal/workloadmeta"
	"github.com/daiyuang/orch/pkg/oopsx"
)

// DeleteDeployEndpoint serves DELETE /api/v1/deploy/{namespace}/{name}.
type DeleteDeployEndpoint struct {
	tasks            *task.Service
	openAPIAuthApply bool
}

func NewDeleteDeployEndpoint(tasks *task.Service, openAPIAuthApply bool) *DeleteDeployEndpoint {
	return &DeleteDeployEndpoint{tasks: tasks, openAPIAuthApply: openAPIAuthApply}
}

func (e *DeleteDeployEndpoint) EndpointSpec() httpx.EndpointSpec {
	spec := httpx.EndpointSpec{
		Prefix:      "/v1/deploy/{namespace}/{name}",
		Description: "Delete deploy desired state and stop its workloads.",
		Tags:        httpx.Tags("deploy"),
	}
	if e.openAPIAuthApply {
		spec.Security = httpx.SecurityRequirements(httpx.SecurityRequirement("bearerAuth"))
	}
	return spec
}

func (e *DeleteDeployEndpoint) Register(r httpx.Registrar) {
	httpx.MustGroupDelete(r.Scope(), "", e.handle, OpenAPIMeta([]string{"deploy"}, "deleteDeployApp",
		"Delete a deploy app",
		"Stops the app workloads, removes the desired app document, and records stopped scheduler assignments."))
}

func (e *DeleteDeployEndpoint) handle(ctx context.Context, in *DeleteDeployInput) (*DeleteDeployOutput, error) {
	meta := deployv1.Metadata{Name: in.Name, Namespace: in.Namespace}
	if err := e.tasks.SubmitDelete(ctx, meta); err != nil {
		return nil, oopsx.B("api").Wrapf(err, "delete app")
	}
	out := &DeleteDeployOutput{}
	out.Body.Accepted = true
	out.Body.App = meta.Name
	out.Body.Namespace = workloadmeta.NamespaceOrDefault(meta.Namespace)
	out.Body.Status = workloadmeta.AssignmentStatusStopped
	return out, nil
}
