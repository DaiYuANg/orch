package api

import (
	"context"
	"time"

	"github.com/arcgolabs/collectionx/list"
	"github.com/arcgolabs/httpx"

	"github.com/daiyuang/orch/internal/config"
	"github.com/daiyuang/orch/internal/dixdiag"
	"github.com/daiyuang/orch/internal/raftsvc"
)

// ReadyEndpoint serves GET /api/ready.
type ReadyEndpoint struct {
	cfg  config.Config
	raft *raftsvc.Service
	diag *dixdiag.Service
}

func NewReadyEndpoint(cfg config.Config, raft *raftsvc.Service, diag *dixdiag.Service) *ReadyEndpoint {
	return &ReadyEndpoint{cfg: cfg, raft: raft, diag: diag}
}

func (e *ReadyEndpoint) EndpointSpec() httpx.EndpointSpec {
	return httpx.EndpointSpec{
		Prefix:      "/ready",
		Description: "Control-plane readiness.",
		Tags:        httpx.Tags("system"),
	}
}

func (e *ReadyEndpoint) Register(r httpx.Registrar) {
	httpx.MustGroupGet(r.Scope(), "", e.handle, OpenAPIMeta([]string{"system"}, "ready", "Control-plane readiness",
		"Returns whether the local API can serve cluster operations, including Raft leader discovery and write routing."))
}

func (e *ReadyEndpoint) handle(ctx context.Context, _ *EmptyInput) (*ReadyOutput, error) {
	checks := list.NewList[ReadyCheckItem]()
	checks.Add(ReadyCheckItem{Name: "http", Ready: true, Status: "ok"})

	raftReady := false
	writeReady := false
	if e == nil || e.raft == nil {
		checks.Add(ReadyCheckItem{Name: "raft", Ready: false, Status: "not_ready", Detail: "raft service unavailable"})
		checks.Add(ReadyCheckItem{Name: "writes", Ready: false, Status: "not_ready", Detail: "raft service unavailable"})
	} else {
		status, err := e.raft.Status(ctx)
		if err != nil {
			detail := err.Error()
			checks.Add(ReadyCheckItem{Name: "raft", Ready: false, Status: "error", Detail: detail})
			checks.Add(ReadyCheckItem{Name: "writes", Ready: false, Status: "error", Detail: detail})
		} else {
			raftReady = status.Ready && status.LeaderID != ""
			checks.Add(ReadyCheckItem{Name: "raft", Ready: raftReady, Status: status.State, Detail: status.Message})
			writeReady = status.IsLeader
			writeDetail := ""
			if !writeReady {
				if status.LeaderID == "" {
					writeDetail = "leader is not known"
				} else if leaderURL, ok := e.cfg.Cluster.NodeURL(status.LeaderID); ok {
					writeReady = true
					writeDetail = "writes forward to " + leaderURL
				} else {
					writeDetail = "configure cluster.nodes." + status.LeaderID + " or target the leader API"
				}
			}
			checks.Add(ReadyCheckItem{Name: "writes", Ready: writeReady, Status: readyStatus(writeReady), Detail: writeDetail})
		}
	}

	dixReady := true
	if e != nil && e.diag != nil {
		report := e.diag.CheckReadiness(ctx)
		dixReady = report.Healthy()
		addDixHealthItems(checks, "dix/", report)
	}

	ready := raftReady && writeReady && dixReady
	out := &ReadyOutput{}
	out.Body.Ready = ready
	out.Body.Status = readyStatus(ready)
	out.Body.Timestamp = time.Now().UTC().Format(time.RFC3339)
	out.Body.Checks = checks
	return out, nil
}

func readyStatus(ready bool) string {
	if ready {
		return "ready"
	}
	return "not_ready"
}
