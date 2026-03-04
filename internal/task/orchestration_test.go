package task

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMigrateAndFailoverDeployment(t *testing.T) {
	ctx := context.Background()
	registryService := newRegistryForTaskTest(t)
	runtime := newFakeRuntime("mock")
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	service := NewServiceWithRuntimeFactory(logger, registryService, func() (RuntimeExecutor, error) {
		return runtime, nil
	})

	type runCall struct {
		Driver string `json:"driver"`
	}
	remoteRunCalls := 0
	remoteStopCalls := 0
	remoteStatusCalls := 0
	mux := http.NewServeMux()
	mux.HandleFunc("/tasks/internal/run", func(w http.ResponseWriter, r *http.Request) {
		remoteRunCalls++
		var payload runCall
		_ = json.NewDecoder(r.Body).Decode(&payload)
		assert.Equal(t, "mock", payload.Driver)
		_, _ = w.Write([]byte(`{"code":0,"message":"ok","data":{"container_id":"ctr-remote-1","driver":"mock","node_id":"node-b","node_ip":"10.0.0.2"}}`))
	})
	mux.HandleFunc("/tasks/internal/stop", func(w http.ResponseWriter, r *http.Request) {
		remoteStopCalls++
		_, _ = w.Write([]byte(`{"code":0,"message":"ok","data":{"stopped":true}}`))
	})
	mux.HandleFunc("/tasks/internal/status/ctr-remote-1", func(w http.ResponseWriter, r *http.Request) {
		remoteStatusCalls++
		_, _ = w.Write([]byte(`{"code":0,"message":"ok","data":{"container_id":"ctr-remote-1","name":"ctr-remote-1","running":true,"state":"running"}}`))
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	service.nodeAPI = map[string]string{
		"node-b": server.URL,
	}

	workloadYAML := `
name: migrate-demo
units:
  - name: backend
    tasks:
      - name: api
        type: service
        driver: docker
        image: nginx:latest
        replicas: 1
        network:
          port:
            http: 18080
`

	deployed, err := service.Deploy(ctx, DeployRequest{
		Filename: "migrate-demo.yaml",
		Content:  workloadYAML,
	})
	require.NoError(t, err)
	before, ok := service.GetDeployment(deployed.DeploymentID)
	require.True(t, ok)
	require.Len(t, before.Instances, 1)
	oldContainerID := before.Instances[0].ContainerID

	migrated, err := service.MigrateDeployment(ctx, deployed.DeploymentID, MigrateDeploymentRequest{
		TargetNode: "node-b",
	})
	require.NoError(t, err)
	assert.Equal(t, 1, migrated.Migrated)
	assert.Equal(t, "node-b", migrated.ToNode)
	assert.Equal(t, 1, remoteRunCalls)

	afterMigrate, ok := service.GetDeployment(deployed.DeploymentID)
	require.True(t, ok)
	require.Len(t, afterMigrate.Instances, 1)
	assert.Equal(t, "node-b", afterMigrate.Instances[0].NodeID)
	assert.Equal(t, "ctr-remote-1", afterMigrate.Instances[0].ContainerID)
	assert.Equal(t, "node-b", afterMigrate.Deployment.WorkerNode)
	assert.Contains(t, runtime.stopCalls, oldContainerID)

	failover, err := service.Failover(ctx, FailoverRequest{
		FailedNode: "node-b",
		TargetNode: service.nodeID,
	})
	require.NoError(t, err)
	assert.Equal(t, 1, failover.Deployments)
	assert.Equal(t, 1, remoteStopCalls)
	assert.GreaterOrEqual(t, remoteStatusCalls, 0)

	afterFailover, ok := service.GetDeployment(deployed.DeploymentID)
	require.True(t, ok)
	require.Len(t, afterFailover.Instances, 1)
	assert.Equal(t, service.nodeID, afterFailover.Instances[0].NodeID)
	assert.Equal(t, service.nodeID, afterFailover.Deployment.WorkerNode)
}
