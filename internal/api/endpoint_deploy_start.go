package api

import (
	"context"

	"github.com/arcgolabs/httpx"

	deployv1 "github.com/daiyuang/orch/internal/deploy/v1alpha1"
	"github.com/daiyuang/orch/internal/services/task"
	"github.com/daiyuang/orch/internal/workloadmeta"
	"github.com/daiyuang/orch/pkg/oopsx"
)

// StartDeployEndpoint serves POST /api/v1/deploy/{namespace}/{name}/start.
type StartDeployEndpoint struct {
	tasks            *task.Service
	openAPIAuthApply bool
}

func NewStartDeployEndpoint(tasks *task.Service, openAPIAuthApply bool) *StartDeployEndpoint {
	return &StartDeployEndpoint{tasks: tasks, openAPIAuthApply: openAPIAuthApply}
}

func (e *StartDeployEndpoint) EndpointSpec() httpx.EndpointSpec {
	spec := httpx.EndpointSpec{
		Prefix:      "/v1/deploy/{namespace}/{name}/start",
		Description: "Start deploy workloads from retained desired state.",
		Tags:        httpx.Tags("deploy"),
	}
	if e.openAPIAuthApply {
		spec.Security = httpx.SecurityRequirements(httpx.SecurityRequirement("bearerAuth"))
	}
	return spec
}

func (e *StartDeployEndpoint) Register(r httpx.Registrar) {
	httpx.MustGroupPost(r.Scope(), "", e.handle, OpenAPIMeta([]string{"deploy"}, "startDeployApp",
		"Start a deploy app",
		"Starts the app workloads from its retained desired app document."))
}

func (e *StartDeployEndpoint) handle(ctx context.Context, in *StartDeployInput) (*StartDeployOutput, error) {
	meta := deployv1.Metadata{Name: in.Name, Namespace: in.Namespace}
	if err := e.tasks.SubmitStart(ctx, meta); err != nil {
		return nil, oopsx.B("api").Wrapf(err, "start app")
	}
	out := &StartDeployOutput{}
	out.Body.Accepted = true
	out.Body.App = meta.Name
	out.Body.Namespace = workloadmeta.NamespaceOrDefault(meta.Namespace)
	out.Body.Status = workloadmeta.AssignmentStatusRunning
	return out, nil
}
