package task

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	internalconfig "github.com/DaiYuANg/warden/internal/config"
	"github.com/samber/lo"
	"github.com/samber/mo"
)

const clusterTokenEnv = "WARDEN_CLUSTER_TOKEN"

type apiEnvelope struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

func buildNodeAPIIndex(cfg *internalconfig.Config) map[string]string {
	if cfg == nil {
		return map[string]string{}
	}

	pairs := lo.FilterMap(cfg.Raft.NodeAPI, func(item string, _ int) (lo.Entry[string, string], bool) {
		parts := strings.SplitN(strings.TrimSpace(item), "=", 2)
		if len(parts) != 2 {
			return lo.Entry[string, string]{}, false
		}
		nodeID := strings.TrimSpace(parts[0])
		api := normalizeAPIBase(parts[1])
		if nodeID == "" || api == "" {
			return lo.Entry[string, string]{}, false
		}
		return lo.Entry[string, string]{Key: nodeID, Value: api}, true
	})

	index := lo.Reduce(pairs, func(agg map[string]string, item lo.Entry[string, string], _ int) map[string]string {
		agg[item.Key] = item.Value
		return agg
	}, map[string]string{})

	selfID := strings.TrimSpace(cfg.Raft.NodeID)
	selfAPI := normalizeAPIBase(cfg.Raft.APIAddr)
	if selfAPI == "" && cfg.Http.Port > 0 {
		selfAPI = fmt.Sprintf("http://127.0.0.1:%d", cfg.Http.Port)
	}
	if selfID != "" && selfAPI != "" {
		index[selfID] = selfAPI
	}
	return index
}

func normalizeAPIBase(raw string) string {
	value := strings.TrimSpace(raw)
	if value == "" {
		return ""
	}
	if !strings.Contains(value, "://") {
		value = "http://" + value
	}
	parsed, err := url.Parse(value)
	if err != nil || strings.TrimSpace(parsed.Host) == "" {
		return ""
	}
	parsed.Path = ""
	parsed.RawPath = ""
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return strings.TrimRight(parsed.String(), "/")
}

func resolveIngressAdvertiseIP(cfg *internalconfig.Config, fallback string) string {
	if fallback == "" {
		fallback = "127.0.0.1"
	}
	if cfg == nil {
		return fallback
	}

	listenAddr := strings.TrimSpace(cfg.Network.IngressHTTPListen)
	if listenAddr == "" {
		return fallback
	}

	host, _, err := net.SplitHostPort(listenAddr)
	if err != nil {
		return fallback
	}
	host = strings.TrimSpace(host)
	if host == "" || host == "0.0.0.0" || host == "::" || host == "[::]" {
		return fallback
	}
	if strings.EqualFold(host, "localhost") {
		return "127.0.0.1"
	}
	return host
}

func (s *Service) resolveWorkerAPI(nodeID string) mo.Option[string] {
	id := strings.TrimSpace(nodeID)
	if id == "" {
		return mo.None[string]()
	}
	if api, ok := s.nodeAPI[id]; ok && strings.TrimSpace(api) != "" {
		return mo.Some(api)
	}
	if inferred := normalizeAPIBase(id); inferred != "" {
		return mo.Some(inferred)
	}
	return mo.None[string]()
}

func (s *Service) resolveClusterToken() string {
	if value := strings.TrimSpace(os.Getenv(clusterTokenEnv)); value != "" {
		return value
	}
	defaultPath := filepath.Join(os.TempDir(), "warden.token")
	raw, err := os.ReadFile(defaultPath)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(raw))
}

func (s *Service) runContainerOnWorker(ctx context.Context, workerID string, spec RuntimeRunSpec) (InternalRunResult, error) {
	var result InternalRunResult
	if err := s.workerAPIRequest(ctx, workerID, http.MethodPost, "/tasks/internal/run", InternalRunRequest{
		Spec: spec,
	}, &result); err != nil {
		return InternalRunResult{}, err
	}
	return result, nil
}

func (s *Service) stopContainerOnWorker(ctx context.Context, workerID, containerID string) error {
	return s.workerAPIRequest(ctx, workerID, http.MethodPost, "/tasks/internal/stop", map[string]string{
		"container_id": strings.TrimSpace(containerID),
	}, &map[string]any{})
}

func (s *Service) readContainerLogsOnWorker(ctx context.Context, workerID, containerID string, tail int) (string, error) {
	var payload struct {
		Logs string `json:"logs"`
	}
	path := fmt.Sprintf("/tasks/internal/logs/%s?tail=%d", url.PathEscape(strings.TrimSpace(containerID)), tail)
	if err := s.workerAPIRequest(ctx, workerID, http.MethodGet, path, nil, &payload); err != nil {
		return "", err
	}
	return payload.Logs, nil
}

func (s *Service) workerAPIRequest(ctx context.Context, workerID, method, path string, in any, out any) error {
	base := s.resolveWorkerAPI(workerID)
	if base.IsAbsent() {
		return fmt.Errorf("worker node api is not configured: %s", workerID)
	}

	uri := strings.TrimRight(base.OrEmpty(), "/") + ensureLeadingSlash(path)
	var body io.Reader
	if in != nil {
		raw, err := json.Marshal(in)
		if err != nil {
			return err
		}
		body = bytes.NewReader(raw)
	}

	req, err := http.NewRequestWithContext(ctx, method, uri, body)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	if in != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token := s.resolveClusterToken(); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode >= http.StatusBadRequest {
		return fmt.Errorf("%s %s failed: status=%d body=%s", method, path, resp.StatusCode, strings.TrimSpace(string(raw)))
	}

	if out == nil || len(bytes.TrimSpace(raw)) == 0 {
		return nil
	}

	var envelope apiEnvelope
	if err := json.Unmarshal(raw, &envelope); err == nil && (envelope.Code != 0 || len(envelope.Data) > 0 || envelope.Message != "") {
		if envelope.Code != 0 {
			return fmt.Errorf("api error: code=%d message=%s", envelope.Code, envelope.Message)
		}
		if len(envelope.Data) == 0 {
			return nil
		}
		return json.Unmarshal(envelope.Data, out)
	}

	return json.Unmarshal(raw, out)
}

func ensureLeadingSlash(path string) string {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return "/"
	}
	if strings.HasPrefix(trimmed, "/") {
		return trimmed
	}
	return "/" + trimmed
}
