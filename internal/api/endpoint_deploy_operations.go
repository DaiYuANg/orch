package api

import (
	"context"
	"net/url"

	"github.com/arcgolabs/httpx"

	deployv1 "github.com/daiyuang/orch/internal/deploy/v1alpha1"
	"github.com/daiyuang/orch/internal/services/task"
	"github.com/daiyuang/orch/pkg/oopsx"
)

type MigrateDeployEndpoint struct {
	tasks            *task.Service
	leader           *LeaderForwarder
	openAPIAuthApply bool
}

func NewMigrateDeployEndpoint(tasks *task.Service, leader *LeaderForwarder, openAPIAuthApply bool) *MigrateDeployEndpoint {
	return &MigrateDeployEndpoint{tasks: tasks, leader: leader, openAPIAuthApply: openAPIAuthApply}
}

func (e *MigrateDeployEndpoint) EndpointSpec() httpx.EndpointSpec {
	spec := httpx.EndpointSpec{
		Prefix:      "/v1/deploy/{namespace}/{name}/migrate",
		Description: "Move deploy workloads to a target node.",
		Tags:        httpx.Tags("deploy"),
	}
	if e.openAPIAuthApply {
		spec.Security = httpx.SecurityRequirements(httpx.SecurityRequirement("bearerAuth"))
	}
	return spec
}

func (e *MigrateDeployEndpoint) Register(r httpx.Registrar) {
	httpx.MustGroupPost(r.Scope(), "", e.handle, OpenAPIMeta([]string{"deploy"}, "migrateDeployApp",
		"Migrate deploy app",
		"Stops selected workloads and starts them on targetNode while keeping desired state. Followers forward to the known leader when configured."))
}

func (e *MigrateDeployEndpoint) handle(ctx context.Context, in *DeployOperationInput) (*DeployOperationOutput, error) {
	return handleDeployOperation(ctx, e.tasks, e.leader, in, PathV1DeployMigrate, task.OperationMigrate)
}

type FailoverDeployEndpoint struct {
	tasks            *task.Service
	leader           *LeaderForwarder
	openAPIAuthApply bool
}

func NewFailoverDeployEndpoint(tasks *task.Service, leader *LeaderForwarder, openAPIAuthApply bool) *FailoverDeployEndpoint {
	return &FailoverDeployEndpoint{tasks: tasks, leader: leader, openAPIAuthApply: openAPIAuthApply}
}

func (e *FailoverDeployEndpoint) EndpointSpec() httpx.EndpointSpec {
	spec := httpx.EndpointSpec{
		Prefix:      "/v1/deploy/{namespace}/{name}/failover",
		Description: "Move failed deploy workloads to another node.",
		Tags:        httpx.Tags("deploy"),
	}
	if e.openAPIAuthApply {
		spec.Security = httpx.SecurityRequirements(httpx.SecurityRequirement("bearerAuth"))
	}
	return spec
}

func (e *FailoverDeployEndpoint) Register(r httpx.Registrar) {
	httpx.MustGroupPost(r.Scope(), "", e.handle, OpenAPIMeta([]string{"deploy"}, "failoverDeployApp",
		"Fail over deploy app",
		"Moves failed workloads, or selected workloads, to targetNode or another available node. Followers forward to the known leader when configured."))
}

func (e *FailoverDeployEndpoint) handle(ctx context.Context, in *DeployOperationInput) (*DeployOperationOutput, error) {
	return handleDeployOperation(ctx, e.tasks, e.leader, in, PathV1DeployFailover, task.OperationFailover)
}

type RebalanceDeployEndpoint struct {
	tasks            *task.Service
	leader           *LeaderForwarder
	openAPIAuthApply bool
}

func NewRebalanceDeployEndpoint(tasks *task.Service, leader *LeaderForwarder, openAPIAuthApply bool) *RebalanceDeployEndpoint {
	return &RebalanceDeployEndpoint{tasks: tasks, leader: leader, openAPIAuthApply: openAPIAuthApply}
}

func (e *RebalanceDeployEndpoint) EndpointSpec() httpx.EndpointSpec {
	spec := httpx.EndpointSpec{
		Prefix:      "/v1/deploy/{namespace}/{name}/rebalance",
		Description: "Re-run placement and move workloads whose selected node changes.",
		Tags:        httpx.Tags("deploy"),
	}
	if e.openAPIAuthApply {
		spec.Security = httpx.SecurityRequirements(httpx.SecurityRequirement("bearerAuth"))
	}
	return spec
}

func (e *RebalanceDeployEndpoint) Register(r httpx.Registrar) {
	httpx.MustGroupPost(r.Scope(), "", e.handle, OpenAPIMeta([]string{"deploy"}, "rebalanceDeployApp",
		"Rebalance deploy app",
		"Re-runs placement for selected workloads and migrates only those whose target node changes. Followers forward to the known leader when configured."))
}

func (e *RebalanceDeployEndpoint) handle(ctx context.Context, in *DeployOperationInput) (*DeployOperationOutput, error) {
	return handleDeployOperation(ctx, e.tasks, e.leader, in, PathV1DeployRebalance, task.OperationRebalance)
}

func handleDeployOperation(ctx context.Context, tasks *task.Service, leader *LeaderForwarder, in *DeployOperationInput, basePath, operation string) (*DeployOperationOutput, error) {
	meta := deployv1.Metadata{Name: in.Name, Namespace: in.Namespace}
	out := &DeployOperationOutput{}
	path := basePath + "/" + url.PathEscape(meta.Namespace) + "/" + url.PathEscape(meta.Name) + "/" + operation
	if forwarded, err := leader.ForwardPost(ctx, path, &in.Body, &out.Body); err != nil {
		return nil, oopsx.B("api").Wrapf(err, "forward %s app", operation)
	} else if forwarded {
		return out, nil
	}

	opts := task.AppOperationOptions{
		TargetNode: in.Body.TargetNode,
		Workloads:  in.Body.Workloads,
	}
	var (
		summary task.AppOperationSummary
		err     error
	)
	switch operation {
	case task.OperationMigrate:
		summary, err = tasks.SubmitMigrate(ctx, meta, opts)
	case task.OperationFailover:
		summary, err = tasks.SubmitFailover(ctx, meta, opts)
	case task.OperationRebalance:
		summary, err = tasks.SubmitRebalance(ctx, meta, opts)
	default:
		err = oopsx.B("api").Errorf("unknown deploy operation %q", operation)
	}
	if err != nil {
		return nil, oopsx.B("api").Wrapf(err, "%s app", operation)
	}
	fillDeployOperationOutput(out, summary)
	return out, nil
}

func fillDeployOperationOutput(out *DeployOperationOutput, summary task.AppOperationSummary) {
	out.Body.Accepted = true
	out.Body.Operation = summary.Operation
	out.Body.App = summary.App
	out.Body.Namespace = summary.Namespace
	out.Body.TargetNode = summary.TargetNode
	out.Body.Workloads = summary.Workloads
	out.Body.Moved = summary.Moved
	out.Body.Status = summary.Status
}
