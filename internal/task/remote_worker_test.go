package task

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWorkerAPIClient(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/tasks/internal/run", func(w http.ResponseWriter, r *http.Request) {
		var payload InternalRunRequest
		require.NoError(t, json.NewDecoder(r.Body).Decode(&payload))
		assert.Equal(t, "nginx:latest", payload.Spec.Image)
		_, _ = io.WriteString(w, `{"code":0,"message":"ok","data":{"container_id":"ctr-remote","driver":"docker","node_id":"node-b","node_ip":"10.0.0.2"}}`)
	})
	mux.HandleFunc("/tasks/internal/stop", func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"code":0,"message":"ok","data":{"stopped":true}}`)
	})
	mux.HandleFunc("/tasks/internal/logs/ctr-remote", func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"code":0,"message":"ok","data":{"logs":"remote-log"}}`)
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	service := &Service{
		logger:     slog.New(slog.NewTextHandler(io.Discard, nil)),
		nodeID:     "node-a",
		nodeAPI:    map[string]string{"node-b": server.URL},
		httpClient: &http.Client{Timeout: 3 * time.Second},
	}

	result, err := service.runContainerOnWorker(context.Background(), "node-b", RuntimeRunSpec{
		Image: "nginx:latest",
	})
	require.NoError(t, err)
	assert.Equal(t, "ctr-remote", result.ContainerID)
	assert.Equal(t, "node-b", result.NodeID)

	require.NoError(t, service.stopContainerOnWorker(context.Background(), "node-b", "ctr-remote"))

	logs, err := service.readContainerLogsOnWorker(context.Background(), "node-b", "ctr-remote", 20)
	require.NoError(t, err)
	assert.Equal(t, "remote-log", logs)
}
