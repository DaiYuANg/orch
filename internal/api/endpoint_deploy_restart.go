package api

import (
	"context"

	"github.com/arcgolabs/httpx"

	deployv1 "github.com/daiyuang/orch/internal/deploy/v1alpha1"
	"github.com/daiyuang/orch/internal/services/task"
	"github.com/daiyuang/orch/internal/workloadmeta"
	"github.com/daiyuang/orch/pkg/oopsx"
)

// RestartDeployEndpoint serves POST /api/v1/deploy/{namespace}/{name}/restart.
type RestartDeployEndpoint struct {
	tasks            *task.Service
	openAPIAuthApply bool
}

func NewRestartDeployEndpoint(tasks *task.Service, openAPIAuthApply bool) *RestartDeployEndpoint {
	return &RestartDeployEndpoint{tasks: tasks, openAPIAuthApply: openAPIAuthApply}
}

func (e *RestartDeployEndpoint) EndpointSpec() httpx.EndpointSpec {
	spec := httpx.EndpointSpec{
		Prefix:      "/v1/deploy/{namespace}/{name}/restart",
		Description: "Restart deploy workloads from retained desired state.",
		Tags:        httpx.Tags("deploy"),
	}
	if e.openAPIAuthApply {
		spec.Security = httpx.SecurityRequirements(httpx.SecurityRequirement("bearerAuth"))
	}
	return spec
}

func (e *RestartDeployEndpoint) Register(r httpx.Registrar) {
	httpx.MustGroupPost(r.Scope(), "", e.handle, OpenAPIMeta([]string{"deploy"}, "restartDeployApp",
		"Restart a deploy app",
		"Stops then starts the app workloads using its retained desired app document."))
}

func (e *RestartDeployEndpoint) handle(ctx context.Context, in *RestartDeployInput) (*RestartDeployOutput, error) {
	meta := deployv1.Metadata{Name: in.Name, Namespace: in.Namespace}
	if err := e.tasks.SubmitRestart(ctx, meta); err != nil {
		return nil, oopsx.B("api").Wrapf(err, "restart app")
	}
	out := &RestartDeployOutput{}
	out.Body.Accepted = true
	out.Body.App = meta.Name
	out.Body.Namespace = workloadmeta.NamespaceOrDefault(meta.Namespace)
	out.Body.Status = workloadmeta.AssignmentStatusRunning
	return out, nil
}
