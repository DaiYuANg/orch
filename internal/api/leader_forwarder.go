package api

import (
	"context"
	"net/http"
	"strings"
	"time"

	clientcodec "github.com/arcgolabs/clientx/codec"
	clientxhttp "github.com/arcgolabs/clientx/http"

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
	cfg  config.Config
	raft raftStatusProvider
}

func NewLeaderForwarder(cfg config.Config, raft raftStatusProvider) *LeaderForwarder {
	return &LeaderForwarder{
		cfg:  cfg,
		raft: raft,
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

	var opts []clientxhttp.Option
	if tok := strings.TrimSpace(f.cfg.Cluster.WorkerToken); tok != "" {
		opts = append(opts, clientxhttp.WithHeader("Authorization", "Bearer "+tok))
	}
	client, err := clientxhttp.New(clientxhttp.Config{
		BaseURL: strings.TrimRight(baseURL, "/"),
		Timeout: 60 * time.Second,
	}, opts...)
	if err != nil {
		return true, oopsx.B("api", "raft").Wrapf(err, "create leader HTTP client")
	}
	defer func() { _ = client.Close() }()

	req := client.R()
	if body != nil {
		raw, err := clientcodec.JSON.Marshal(body)
		if err != nil {
			return true, oopsx.B("api", "raft").Wrapf(err, "encode forwarded request")
		}
		req.SetHeader("Content-Type", "application/json").SetBody(raw)
	}

	resp, err := client.Execute(ctx, req, method, path)
	if err != nil {
		return true, oopsx.B("api", "raft").Wrapf(err, "forward request to raft leader")
	}
	if !resp.IsSuccess() {
		msg := strings.TrimSpace(string(resp.Bytes()))
		return true, oopsx.B("api", "raft").Errorf("forwarded request failed: %s: %s", resp.Status(), msg)
	}
	if out != nil && len(resp.Bytes()) > 0 {
		if err := clientcodec.JSON.Unmarshal(resp.Bytes(), out); err != nil {
			return true, oopsx.B("api", "raft").Wrapf(err, "decode forwarded response")
		}
	}
	return true, nil
}
