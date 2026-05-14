package api

import (
	"context"

	"github.com/arcgolabs/httpx"

	"github.com/lyonbrown4d/orch/internal/services/task"
	"github.com/lyonbrown4d/orch/pkg/oopsx"
)

// DeployEndpoint serves POST /api/v1/deploy.
type DeployEndpoint struct {
	tasks            *task.Service
	leader           *LeaderForwarder
	openAPIAuthApply bool // when true, OpenAPI documents bearerAuth on this route (matches Fiber auth when server auth is enabled)
}

func NewDeployEndpoint(tasks *task.Service, leader *LeaderForwarder, openAPIAuthApply bool) *DeployEndpoint {
	return &DeployEndpoint{tasks: tasks, leader: leader, openAPIAuthApply: openAPIAuthApply}
}

func (e *DeployEndpoint) EndpointSpec() httpx.EndpointSpec {
	spec := httpx.EndpointSpec{
		Prefix:      "/v1/deploy",
		Description: "Apply deploy DSL to the control plane.",
		Tags:        httpx.Tags("deploy"),
	}
	if e.openAPIAuthApply {
		spec.Security = httpx.SecurityRequirements(httpx.SecurityRequirement("bearerAuth"))
	}
	return spec
}

func (e *DeployEndpoint) Register(r httpx.Registrar) {
	httpx.MustGroupPost(r.Scope(), "", e.handle, OpenAPIMeta([]string{"deploy"}, "deployApp", "Apply a deploy DSL document (submit workload desired state)",
		"Accepts an orch deploy v1alpha1 document in JSON. When server auth is enabled, requires a valid JWT bearer token."))
}

func (e *DeployEndpoint) handle(ctx context.Context, in *DeployInput) (*DeployOutput, error) {
	out := &DeployOutput{}
	if forwarded, err := e.leader.ForwardPost(ctx, PathV1Deploy, &in.Body, &out.Body); err != nil {
		return nil, oopsx.B("api").Wrapf(err, "forward deploy app")
	} else if forwarded {
		return out, nil
	}
	if err := e.tasks.SubmitDeploy(ctx, &in.Body); err != nil {
		return nil, oopsx.B("api").Wrapf(err, "deploy app")
	}
	out.Body.Accepted = true
	out.Body.App = in.Body.Metadata.Name
	out.Body.Workloads = len(in.Body.Workloads)
	return out, nil
}
