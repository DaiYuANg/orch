package api

import (
	"context"

	"github.com/arcgolabs/httpx"

	"github.com/daiyuang/orch/internal/services/task"
	"github.com/daiyuang/orch/pkg/oopsx"
)

// DeployEndpoint serves POST /api/v1/deploy.
type DeployEndpoint struct {
	tasks *task.Service
}

func NewDeployEndpoint(tasks *task.Service) *DeployEndpoint {
	return &DeployEndpoint{tasks: tasks}
}

func (e *DeployEndpoint) EndpointSpec() httpx.EndpointSpec {
	return httpx.EndpointSpec{
		Prefix:      "/v1/deploy",
		Description: "Apply deploy DSL to the control plane.",
		Tags:        httpx.Tags("deploy"),
	}
}

func (e *DeployEndpoint) Register(r httpx.Registrar) {
	httpx.MustGroupPost(r.Scope(), "", e.handle, OpenAPIMeta([]string{"deploy"}, "deployApp", "Apply a deploy DSL document (submit workload desired state)"))
}

func (e *DeployEndpoint) handle(ctx context.Context, in *DeployInput) (*DeployOutput, error) {
	if err := e.tasks.DeployApp(ctx, &in.Body); err != nil {
		return nil, oopsx.B("api").Wrapf(err, "deploy app")
	}
	out := &DeployOutput{}
	out.Body.Accepted = true
	out.Body.App = in.Body.Metadata.Name
	out.Body.Workloads = len(in.Body.Workloads)
	return out, nil
}
