package task

import (
	"context"
	"strings"
	"time"

	"github.com/arcgolabs/clientx"
	clientcodec "github.com/arcgolabs/clientx/codec"
	clientxhttp "github.com/arcgolabs/clientx/http"

	"github.com/daiyuang/orch/internal/config"
	deployv1 "github.com/daiyuang/orch/internal/deploy/v1alpha1"
	"github.com/daiyuang/orch/internal/workerapi"
	"github.com/daiyuang/orch/pkg/oopsx"
)

type WorkerDispatcher interface {
	DispatchWorkload(ctx context.Context, nodeID string, meta deployv1.Metadata, workload deployv1.Workload) (DispatchResult, error)
}

type DispatchResult struct {
	Accepted bool
	Node     string
	Status   string
	Workload string
}

type HTTPWorkerDispatcher struct {
	cfg config.Config
}

func NewHTTPWorkerDispatcher(cfg config.Config) *HTTPWorkerDispatcher {
	return &HTTPWorkerDispatcher{cfg: cfg}
}

func (d *HTTPWorkerDispatcher) DispatchWorkload(ctx context.Context, nodeID string, meta deployv1.Metadata, workload deployv1.Workload) (DispatchResult, error) {
	baseURL, ok := d.cfg.Cluster.NodeURL(nodeID)
	if !ok {
		return DispatchResult{}, oopsx.B("task", "worker").Errorf("no worker API URL configured for node %q (set cluster.nodes.%s)", nodeID, nodeID)
	}

	opts := []clientxhttp.Option{}
	if tok := strings.TrimSpace(d.cfg.Cluster.WorkerToken); tok != "" {
		opts = append(opts, clientxhttp.WithHeader("Authorization", "Bearer "+tok))
	}
	hc, err := clientxhttp.New(clientxhttp.Config{
		BaseURL: baseURL,
		Timeout: 60 * time.Second,
		Retry: clientx.RetryConfig{
			Enabled:    true,
			MaxRetries: 2,
			WaitMin:    200 * time.Millisecond,
			WaitMax:    2 * time.Second,
		},
	}, opts...)
	if err != nil {
		return DispatchResult{}, oopsx.B("task", "worker").Wrapf(err, "create worker client for node %q", nodeID)
	}
	defer func() { _ = hc.Close() }()

	in := workerapi.DeployWorkloadInput{
		Body: workerapi.DeployWorkloadBody{
			Metadata: meta,
			Workload: workload,
			Node:     nodeID,
		},
	}
	raw, err := clientcodec.JSON.Marshal(in.Body)
	if err != nil {
		return DispatchResult{}, oopsx.B("task", "worker").Wrapf(err, "encode worker deploy")
	}
	req := hc.R().
		SetHeader("Content-Type", "application/json").
		SetBody(raw)
	resp, err := hc.Execute(ctx, req, "POST", workerapi.PathV1WorkerDeploy)
	if err != nil {
		return DispatchResult{}, oopsx.B("task", "worker").Wrapf(err, "dispatch workload %q to node %q", workload.Name, nodeID)
	}
	if !resp.IsSuccess() {
		msg := strings.TrimSpace(string(resp.Bytes()))
		return DispatchResult{}, oopsx.B("task", "worker").Errorf("dispatch workload %q to node %q: %s: %s", workload.Name, nodeID, resp.Status(), msg)
	}
	var out workerapi.DeployWorkloadOutput
	if err := clientcodec.JSON.Unmarshal(resp.Bytes(), &out.Body); err != nil {
		return DispatchResult{}, oopsx.B("task", "worker").Wrapf(err, "decode worker deploy response")
	}
	status := strings.TrimSpace(out.Body.Status)
	if status == "" && out.Body.Accepted {
		status = "running"
	}
	node := strings.TrimSpace(out.Body.Node)
	if node == "" {
		node = nodeID
	}
	workloadName := strings.TrimSpace(out.Body.Workload)
	if workloadName == "" {
		workloadName = workload.Name
	}
	return DispatchResult{
		Accepted: out.Body.Accepted,
		Node:     node,
		Status:   status,
		Workload: workloadName,
	}, nil
}
