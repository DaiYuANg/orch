package api

import (
	"context"
	"net/url"

	"github.com/arcgolabs/httpx"

	deployv1 "github.com/daiyuang/orch/internal/deploy/v1alpha1"
	"github.com/daiyuang/orch/internal/services/task"
	"github.com/daiyuang/orch/internal/workloadmeta"
	"github.com/daiyuang/orch/pkg/oopsx"
)

// RestartDeployEndpoint serves POST /api/v1/deploy/{namespace}/{name}/restart.
type RestartDeployEndpoint struct {
	tasks            *task.Service
	leader           *LeaderForwarder
	openAPIAuthApply bool
}

func NewRestartDeployEndpoint(tasks *task.Service, leader *LeaderForwarder, openAPIAuthApply bool) *RestartDeployEndpoint {
	return &RestartDeployEndpoint{tasks: tasks, leader: leader, openAPIAuthApply: openAPIAuthApply}
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
	out := &RestartDeployOutput{}
	path := PathV1DeployRestart + "/" + url.PathEscape(meta.Namespace) + "/" + url.PathEscape(meta.Name) + "/restart"
	if forwarded, err := e.leader.ForwardPost(ctx, path, struct{}{}, &out.Body); err != nil {
		return nil, oopsx.B("api").Wrapf(err, "forward restart app")
	} else if forwarded {
		return out, nil
	}
	if err := e.tasks.SubmitRestart(ctx, meta); err != nil {
		return nil, oopsx.B("api").Wrapf(err, "restart app")
	}
	out.Body.Accepted = true
	out.Body.App = meta.Name
	out.Body.Namespace = workloadmeta.NamespaceOrDefault(meta.Namespace)
	out.Body.Status = workloadmeta.AssignmentStatusRunning
	return out, nil
}
