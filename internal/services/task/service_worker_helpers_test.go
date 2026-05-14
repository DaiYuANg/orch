package task_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/lyonbrown4d/orch/internal/workerapi"
)

func newDeployWorkerServer(t *testing.T, dispatchCh chan<- workerapi.DeployWorkloadBody, status string) *httptest.Server {
	t.Helper()
	worker := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requireWorkerPath(t, r, workerapi.PathV1WorkerDeploy)
		in := decodeDeployRequest(t, r)
		dispatchCh <- in
		out := workerapi.DeployWorkloadOutput{}
		out.Body.Accepted = true
		out.Body.Node = in.Node
		out.Body.Status = status
		out.Body.Workload = in.Workload.Name
		writeWorkerResponse(t, w, out.Body)
	}))
	t.Cleanup(worker.Close)
	return worker
}

func newFailingDeployWorkerServer(t *testing.T, dispatchCh chan<- struct{}) *httptest.Server {
	t.Helper()
	worker := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requireWorkerPath(t, r, workerapi.PathV1WorkerDeploy)
		select {
		case dispatchCh <- struct{}{}:
		default:
		}
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	t.Cleanup(worker.Close)
	return worker
}

func requireWorkerPath(t *testing.T, r *http.Request, want string) {
	t.Helper()
	if r.URL.Path != want {
		t.Fatalf("worker path = %q, want %q", r.URL.Path, want)
	}
}

func decodeDeployRequest(t *testing.T, r *http.Request) workerapi.DeployWorkloadBody {
	t.Helper()
	var in workerapi.DeployWorkloadBody
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		t.Fatalf("decode worker request: %v", err)
	}
	return in
}

func writeWorkerResponse(t *testing.T, w http.ResponseWriter, v any) {
	t.Helper()
	if err := json.NewEncoder(w).Encode(v); err != nil {
		t.Fatalf("encode worker response: %v", err)
	}
}

func waitWorkerDispatch(t *testing.T, dispatchCh <-chan workerapi.DeployWorkloadBody, timeout time.Duration) workerapi.DeployWorkloadBody {
	t.Helper()
	select {
	case got := <-dispatchCh:
		return got
	case <-time.After(timeout):
		t.Fatal("timed out waiting for worker dispatch")
		return workerapi.DeployWorkloadBody{}
	}
}

func requireWorkerDispatch(t *testing.T, got workerapi.DeployWorkloadBody, nodeID, workloadName string) {
	t.Helper()
	if got.Node != nodeID || got.Workload.Name != workloadName {
		t.Fatalf("dispatch request = %#v", got)
	}
}
