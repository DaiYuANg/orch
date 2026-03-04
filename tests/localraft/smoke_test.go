package localraft

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/miekg/dns"
	"github.com/stretchr/testify/require"
)

type apiEnvelope[T any] struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    T      `json:"data"`
}

type deployResult struct {
	DeploymentID string `json:"deployment_id"`
}

type deploymentDetail struct {
	Deployment struct {
		ID          string `json:"id"`
		DesiredNode string `json:"desired_node"`
		WorkerNode  string `json:"worker_node"`
	} `json:"deployment"`
	Instances []struct {
		ID     string `json:"id"`
		NodeID string `json:"node_id"`
	} `json:"instances"`
}

func TestLocalRaftSmoke(t *testing.T) {
	if strings.TrimSpace(os.Getenv("WARDEN_LOCAL_RAFT_SMOKE")) != "1" {
		t.Skip("set WARDEN_LOCAL_RAFT_SMOKE=1 to run local raft smoke test")
	}

	leaderAPI := envOrDefault("WARDEN_SMOKE_LEADER_API", "http://127.0.0.1:7443")
	followerAPI := envOrDefault("WARDEN_SMOKE_FOLLOWER_API", "http://127.0.0.1:7444")
	ingressAddr := envOrDefault("WARDEN_SMOKE_INGRESS", "127.0.0.1:18082")
	dnsAddr := envOrDefault("WARDEN_SMOKE_DNS", "127.0.0.1:10532")

	workloadPath := filepath.Join("..", "..", "examples", "local-raft", "echo.yaml")
	content, err := os.ReadFile(workloadPath)
	require.NoError(t, err)

	client := &http.Client{Timeout: 5 * time.Second}

	var deploy deployResult
	err = postJSON(client, leaderAPI+"/tasks/deploy", map[string]any{
		"filename": "echo.yaml",
		"content":  string(content),
	}, &deploy)
	require.NoError(t, err)
	require.NotEmpty(t, deploy.DeploymentID)

	t.Cleanup(func() {
		_ = postJSON(client, leaderAPI+"/tasks/"+deploy.DeploymentID+"/stop", map[string]any{}, &map[string]any{})
	})

	var detail deploymentDetail
	require.Eventually(t, func() bool {
		getErr := getJSON(client, leaderAPI+"/tasks/"+deploy.DeploymentID, &detail)
		return getErr == nil && detail.Deployment.ID == deploy.DeploymentID && len(detail.Instances) > 0
	}, 20*time.Second, 500*time.Millisecond)

	require.NotEmpty(t, strings.TrimSpace(detail.Deployment.DesiredNode))
	require.NotEmpty(t, strings.TrimSpace(detail.Deployment.WorkerNode))
	require.Equal(t, detail.Deployment.WorkerNode, detail.Instances[0].NodeID)

	require.Eventually(t, func() bool {
		req, reqErr := http.NewRequestWithContext(context.Background(), http.MethodGet, "http://"+ingressAddr+"/", nil)
		if reqErr != nil {
			return false
		}
		req.Host = "echo.warden.local"
		resp, doErr := client.Do(req)
		if doErr != nil {
			return false
		}
		defer resp.Body.Close()
		raw, readErr := io.ReadAll(resp.Body)
		if readErr != nil {
			return false
		}
		return resp.StatusCode == http.StatusOK && strings.Contains(string(raw), "raft-ok")
	}, 20*time.Second, 500*time.Millisecond)

	require.Eventually(t, func() bool {
		msg := new(dns.Msg)
		msg.SetQuestion(dns.Fqdn("echo.warden.local"), dns.TypeA)
		resp, _, dnsErr := new(dns.Client).Exchange(msg, dnsAddr)
		return dnsErr == nil && resp != nil && len(resp.Answer) > 0
	}, 10*time.Second, 500*time.Millisecond)

	var clusterStatus map[string]any
	require.NoError(t, getJSON(client, followerAPI+"/system/cluster", &clusterStatus))
	require.Equal(t, true, clusterStatus["enabled"])
}

func envOrDefault(key, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}

func getJSON[T any](client *http.Client, url string, out *T) error {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	return doJSON(client, req, out)
}

func postJSON[T any](client *http.Client, url string, in any, out *T) error {
	raw, err := json.Marshal(in)
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(raw))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	return doJSON(client, req, out)
}

func doJSON[T any](client *http.Client, req *http.Request, out *T) error {
	tokenPath := filepath.Join(os.TempDir(), "warden.token")
	if raw, err := os.ReadFile(tokenPath); err == nil {
		if token := strings.TrimSpace(string(raw)); token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
	}
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode >= http.StatusBadRequest {
		return fmt.Errorf("%s %s failed: %d %s", req.Method, req.URL.String(), resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var envelope apiEnvelope[T]
	if err := json.Unmarshal(body, &envelope); err != nil {
		return err
	}
	if envelope.Code != 0 {
		return fmt.Errorf("api code=%d message=%s", envelope.Code, envelope.Message)
	}
	*out = envelope.Data
	return nil
}
