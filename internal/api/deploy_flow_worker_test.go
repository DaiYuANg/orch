package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/daiyuang/orch/internal/workerapi"
	"github.com/daiyuang/orch/internal/workloadmeta"
)

func newDeployFlowWorker(
	t *testing.T,
	deploys chan<- workerapi.DeployWorkloadBody,
	stops chan<- workerapi.StopWorkloadBody,
) *httptest.Server {
	t.Helper()
	worker := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handleDeployFlowWorkerRequest(t, w, r, deploys, stops)
	}))
	t.Cleanup(worker.Close)
	return worker
}

func handleDeployFlowWorkerRequest(
	t *testing.T,
	w http.ResponseWriter,
	r *http.Request,
	deploys chan<- workerapi.DeployWorkloadBody,
	stops chan<- workerapi.StopWorkloadBody,
) {
	t.Helper()
	switch r.URL.Path {
	case workerapi.PathV1WorkerDeploy:
		handleDeployFlowWorkerDeploy(t, w, r, deploys)
	case workerapi.PathV1WorkerStop:
		handleDeployFlowWorkerStop(t, w, r, stops)
	default:
		t.Fatalf("worker path = %q", r.URL.Path)
	}
}

func handleDeployFlowWorkerDeploy(t *testing.T, w http.ResponseWriter, r *http.Request, out chan<- workerapi.DeployWorkloadBody) {
	t.Helper()
	var in workerapi.DeployWorkloadBody
	requireNoError(t, json.NewDecoder(r.Body).Decode(&in), "decode worker request")
	out <- in
	resp := workerapi.DeployWorkloadOutput{}
	resp.Body.Accepted = true
	resp.Body.Node = in.Node
	resp.Body.Status = workloadmeta.AssignmentStatusRunning
	resp.Body.Workload = in.Workload.Name
	writeWorkerJSON(t, w, resp.Body)
}

func handleDeployFlowWorkerStop(t *testing.T, w http.ResponseWriter, r *http.Request, out chan<- workerapi.StopWorkloadBody) {
	t.Helper()
	var in workerapi.StopWorkloadBody
	requireNoError(t, json.NewDecoder(r.Body).Decode(&in), "decode worker stop request")
	out <- in
	resp := workerapi.StopWorkloadOutput{}
	resp.Body.Accepted = true
	resp.Body.Node = in.Node
	resp.Body.Status = workloadmeta.AssignmentStatusStopped
	resp.Body.Workload = in.Workload.Name
	writeWorkerJSON(t, w, resp.Body)
}

func writeWorkerJSON(t *testing.T, w http.ResponseWriter, v any) {
	t.Helper()
	requireNoError(t, json.NewEncoder(w).Encode(v), "encode worker response")
}
