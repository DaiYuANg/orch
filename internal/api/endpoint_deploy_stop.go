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

// StopDeployEndpoint serves POST /api/v1/deploy/{namespace}/{name}/stop.
type StopDeployEndpoint struct {
	tasks            *task.Service
	leader           *LeaderForwarder
	openAPIAuthApply bool
}

func NewStopDeployEndpoint(tasks *task.Service, leader *LeaderForwarder, openAPIAuthApply bool) *StopDeployEndpoint {
	return &StopDeployEndpoint{tasks: tasks, leader: leader, openAPIAuthApply: openAPIAuthApply}
}

func (e *StopDeployEndpoint) EndpointSpec() httpx.EndpointSpec {
	spec := httpx.EndpointSpec{
		Prefix:      "/v1/deploy/{namespace}/{name}/stop",
		Description: "Stop deploy workloads while keeping desired state.",
		Tags:        httpx.Tags("deploy"),
	}
	if e.openAPIAuthApply {
		spec.Security = httpx.SecurityRequirements(httpx.SecurityRequirement("bearerAuth"))
	}
	return spec
}

func (e *StopDeployEndpoint) Register(r httpx.Registrar) {
	httpx.MustGroupPost(r.Scope(), "", e.handle, OpenAPIMeta([]string{"deploy"}, "stopDeployApp",
		"Stop a deploy app",
		"Stops the app workloads and records stopped scheduler assignments without deleting the desired app document."))
}

func (e *StopDeployEndpoint) handle(ctx context.Context, in *StopDeployInput) (*StopDeployOutput, error) {
	meta := deployv1.Metadata{Name: in.Name, Namespace: in.Namespace}
	out := &StopDeployOutput{}
	path := PathV1DeployStop + "/" + url.PathEscape(meta.Namespace) + "/" + url.PathEscape(meta.Name) + "/stop"
	if forwarded, err := e.leader.ForwardPost(ctx, path, struct{}{}, &out.Body); err != nil {
		return nil, oopsx.B("api").Wrapf(err, "forward stop app")
	} else if forwarded {
		return out, nil
	}
	if err := e.tasks.SubmitStop(ctx, meta); err != nil {
		return nil, oopsx.B("api").Wrapf(err, "stop app")
	}
	out.Body.Accepted = true
	out.Body.App = meta.Name
	out.Body.Namespace = workloadmeta.NamespaceOrDefault(meta.Namespace)
	out.Body.Status = workloadmeta.AssignmentStatusStopped
	return out, nil
}
