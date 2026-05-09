package api

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/daiyuang/orch/internal/config"
	"github.com/daiyuang/orch/internal/raftsvc"
	"github.com/daiyuang/orch/pkg/oopsx"
)

type raftStatusProvider interface {
	Status(context.Context) (raftsvc.Status, error)
}

// LeaderForwarder forwards write requests received by a Raft follower to the
// current leader's HTTP API when cluster.nodes contains the leader node ID.
type LeaderForwarder struct {
	cfg    config.Config
	raft   raftStatusProvider
	client *http.Client
}

func NewLeaderForwarder(cfg config.Config, raft raftStatusProvider) *LeaderForwarder {
	return &LeaderForwarder{
		cfg:  cfg,
		raft: raft,
		client: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

func (f *LeaderForwarder) ForwardPost(ctx context.Context, path string, body, out any) (bool, error) {
	return f.forwardJSON(ctx, http.MethodPost, path, body, out)
}

func (f *LeaderForwarder) ForwardDelete(ctx context.Context, path string, out any) (bool, error) {
	return f.forwardJSON(ctx, http.MethodDelete, path, nil, out)
}

func (f *LeaderForwarder) leaderBaseURL(ctx context.Context) (string, bool, error) {
	if f == nil || f.raft == nil {
		return "", false, nil
	}
	status, err := f.raft.Status(ctx)
	if err != nil {
		return "", false, err
	}
	if !status.Ready || status.IsLeader {
		return "", false, nil
	}
	leaderID := strings.TrimSpace(status.LeaderID)
	if leaderID == "" {
		return "", false, oopsx.B("api", "raft").Errorf("not leader and raft leader is not known")
	}
	baseURL, ok := f.cfg.Cluster.NodeURL(leaderID)
	if !ok {
		return "", false, oopsx.B("api", "raft").Errorf("not leader: configure cluster.nodes.%s or target the raft leader", leaderID)
	}
	return baseURL, true, nil
}

func (f *LeaderForwarder) forwardJSON(ctx context.Context, method, path string, body, out any) (bool, error) {
	baseURL, ok, err := f.leaderBaseURL(ctx)
	if err != nil || !ok {
		return ok, err
	}

	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return true, oopsx.B("api", "raft").Wrapf(err, "encode forwarded request")
		}
		reader = bytes.NewReader(raw)
	}
	req, err := http.NewRequestWithContext(ctx, method, strings.TrimRight(baseURL, "/")+path, reader)
	if err != nil {
		return true, oopsx.B("api", "raft").Wrapf(err, "build forwarded request")
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if tok := strings.TrimSpace(f.cfg.Cluster.WorkerToken); tok != "" {
		req.Header.Set("Authorization", "Bearer "+tok)
	}

	client := f.client
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return true, oopsx.B("api", "raft").Wrapf(err, "forward request to raft leader")
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return true, oopsx.B("api", "raft").Wrapf(err, "read forwarded response")
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		msg := strings.TrimSpace(string(respBody))
		return true, oopsx.B("api", "raft").Errorf("forwarded request failed: %s: %s", resp.Status, msg)
	}
	if out != nil && len(respBody) > 0 {
		if err := json.Unmarshal(respBody, out); err != nil {
			return true, oopsx.B("api", "raft").Wrapf(err, "decode forwarded response")
		}
	}
	return true, nil
}
