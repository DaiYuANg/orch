package task

import (
	"context"
	"log/slog"
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
	StopWorkload(ctx context.Context, nodeID string, meta deployv1.Metadata, workload deployv1.Workload) (DispatchResult, error)
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
	in := workerapi.DeployWorkloadInput{
		Body: workerapi.DeployWorkloadBody{
			Metadata: meta,
			Workload: workload,
			Node:     nodeID,
		},
	}
	return d.executeWorker(ctx, nodeID, workload.Name, "dispatch", workerapi.PathV1WorkerDeploy, in.Body, decodeWorkerDeploy)
}

func (d *HTTPWorkerDispatcher) StopWorkload(ctx context.Context, nodeID string, meta deployv1.Metadata, workload deployv1.Workload) (DispatchResult, error) {
	in := workerapi.StopWorkloadInput{
		Body: workerapi.StopWorkloadBody{
			Metadata: meta,
			Workload: workload,
			Node:     nodeID,
		},
	}
	return d.executeWorker(ctx, nodeID, workload.Name, "stop", workerapi.PathV1WorkerStop, in.Body, decodeWorkerStop)
}

func decodeWorkerDeploy(data []byte, nodeID, fallbackWorkload string) (DispatchResult, error) {
	var out workerapi.DeployWorkloadOutput
	if err := clientcodec.JSON.Unmarshal(data, &out.Body); err != nil {
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
		workloadName = fallbackWorkload
	}
	return DispatchResult{
		Accepted: out.Body.Accepted,
		Node:     node,
		Status:   status,
		Workload: workloadName,
	}, nil
}

func decodeWorkerStop(data []byte, nodeID, fallbackWorkload string) (DispatchResult, error) {
	var out workerapi.StopWorkloadOutput
	if err := clientcodec.JSON.Unmarshal(data, &out.Body); err != nil {
		return DispatchResult{}, oopsx.B("task", "worker").Wrapf(err, "decode worker stop response")
	}
	status := strings.TrimSpace(out.Body.Status)
	if status == "" && out.Body.Accepted {
		status = "stopped"
	}
	node := strings.TrimSpace(out.Body.Node)
	if node == "" {
		node = nodeID
	}
	workloadName := strings.TrimSpace(out.Body.Workload)
	if workloadName == "" {
		workloadName = fallbackWorkload
	}
	return DispatchResult{
		Accepted: out.Body.Accepted,
		Node:     node,
		Status:   status,
		Workload: workloadName,
	}, nil
}

func (d *HTTPWorkerDispatcher) executeWorker(
	ctx context.Context,
	nodeID string,
	workloadName string,
	action string,
	path string,
	body any,
	decode func([]byte, string, string) (DispatchResult, error),
) (DispatchResult, error) {
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
	defer closeWorkerClient(hc)

	raw, err := clientcodec.JSON.Marshal(body)
	if err != nil {
		return DispatchResult{}, oopsx.B("task", "worker").Wrapf(err, "encode worker %s", action)
	}
	req := hc.R().
		SetHeader("Content-Type", "application/json").
		SetBody(raw)
	resp, err := hc.Execute(ctx, req, "POST", path)
	if err != nil {
		return DispatchResult{}, oopsx.B("task", "worker").Wrapf(err, "%s workload %q on node %q", action, workloadName, nodeID)
	}
	if !resp.IsSuccess() {
		msg := strings.TrimSpace(string(resp.Bytes()))
		return DispatchResult{}, oopsx.B("task", "worker").Errorf("%s workload %q on node %q: %s: %s", action, workloadName, nodeID, resp.Status(), msg)
	}
	return decode(resp.Bytes(), nodeID, workloadName)
}

func closeWorkerClient(hc clientxhttp.Client) {
	if err := hc.Close(); err != nil {
		slog.Default().Debug("worker client close", "error", err)
	}
}
