package api

import (
	"context"
	"strings"

	"github.com/arcgolabs/httpx"

	"github.com/daiyuang/orch/internal/deploy/loader"
	"github.com/daiyuang/orch/internal/services/task"
	"github.com/daiyuang/orch/pkg/oopsx"
)

// DeploySourceEndpoint serves POST /api/v1/deploy/source.
type DeploySourceEndpoint struct {
	loader           *loader.Loader
	tasks            *task.Service
	openAPIAuthApply bool
}

func NewDeploySourceEndpoint(loader *loader.Loader, tasks *task.Service, openAPIAuthApply bool) *DeploySourceEndpoint {
	return &DeploySourceEndpoint{loader: loader, tasks: tasks, openAPIAuthApply: openAPIAuthApply}
}

func (e *DeploySourceEndpoint) EndpointSpec() httpx.EndpointSpec {
	spec := httpx.EndpointSpec{
		Prefix:      "/v1/deploy/source",
		Description: "Apply deploy manifest source (.orch or YAML) to the control plane.",
		Tags:        httpx.Tags("deploy"),
	}
	if e.openAPIAuthApply {
		spec.Security = httpx.SecurityRequirements(httpx.SecurityRequirement("bearerAuth"))
	}
	return spec
}

func (e *DeploySourceEndpoint) Register(r httpx.Registrar) {
	httpx.MustGroupPost(r.Scope(), "", e.handle, OpenAPIMeta([]string{"deploy"}, "deployAppSource",
		"Apply deploy source code",
		"Parses virtualPath suffix (.orch plano, otherwise YAML) on the server, then replicates desired state via Raft. Target the Raft leader when clustering. Requires JWT when auth is enabled."))
}

func (e *DeploySourceEndpoint) handle(ctx context.Context, in *DeploySourceInput) (*DeployOutput, error) {
	vp := strings.TrimSpace(in.Body.VirtualPath)
	if vp == "" {
		return nil, oopsx.B("api").Errorf("virtualPath is required")
	}
	app, err := e.loader.LoadAppString(ctx, vp, in.Body.Source)
	if err != nil {
		return nil, oopsx.B("api").Wrapf(err, "parse deploy source")
	}
	if err := e.tasks.SubmitDeploy(ctx, app); err != nil {
		return nil, oopsx.B("api").Wrapf(err, "deploy app")
	}
	out := &DeployOutput{}
	out.Body.Accepted = true
	out.Body.App = app.Metadata.Name
	out.Body.Workloads = len(app.Workloads)
	return out, nil
}
