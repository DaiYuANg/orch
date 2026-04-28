package api

import (
	"context"
	"time"

	"github.com/arcgolabs/httpx"

	deployv1 "github.com/daiyuang/orch/internal/deploy/v1alpha1"
	"github.com/daiyuang/orch/internal/hostinfo"
	"github.com/daiyuang/orch/internal/services/registry"
	"github.com/daiyuang/orch/internal/services/task"
)

type healthOutput struct {
	Body struct {
		Status    string `json:"status"`
		Timestamp string `json:"timestamp"`
	} `json:"body"`
}

type listWorkloadsOutput struct {
	Body struct {
		Items []registry.WorkloadRecord `json:"items"`
	} `json:"body"`
}

type deployInput struct {
	Body deployv1.App `json:"body"`
}

type deployOutput struct {
	Body struct {
		Accepted  bool   `json:"accepted"`
		App       string `json:"app"`
		Workloads int    `json:"workloads"`
	} `json:"body"`
}

type hostinfoOutput struct {
	Body hostinfo.Report `json:"body"`
}

func Register(s httpx.ServerRuntime, registrySvc *registry.Service, taskSvc *task.Service) {
	httpx.MustGet(s, "/health", func(_ context.Context, _ *struct{}) (*healthOutput, error) {
		out := &healthOutput{}
		out.Body.Status = "ok"
		out.Body.Timestamp = time.Now().UTC().Format(time.RFC3339)
		return out, nil
	})

	v1 := s.Group("/v1")

	httpx.MustGroupGet(v1, "/hostinfo", func(ctx context.Context, _ *struct{}) (*hostinfoOutput, error) {
		snap, err := hostinfo.Collect(ctx)
		if err != nil {
			return nil, err
		}
		out := &hostinfoOutput{}
		out.Body = *snap
		return out, nil
	})

	httpx.MustGroupGet(v1, "/workloads", func(_ context.Context, _ *struct{}) (*listWorkloadsOutput, error) {
		out := &listWorkloadsOutput{}
		out.Body.Items = registrySvc.List()
		return out, nil
	})

	httpx.MustGroupPost(v1, "/deploy", func(ctx context.Context, in *deployInput) (*deployOutput, error) {
		if err := taskSvc.DeployApp(ctx, &in.Body); err != nil {
			return nil, err
		}
		out := &deployOutput{}
		out.Body.Accepted = true
		out.Body.App = in.Body.Metadata.Name
		out.Body.Workloads = len(in.Body.Workloads)
		return out, nil
	})
}
