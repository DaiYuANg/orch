package api

import (
	"context"
	"time"

	"github.com/arcgolabs/collectionx/list"
	"github.com/arcgolabs/httpx"

	"github.com/lyonbrown4d/orch/internal/config"
	"github.com/lyonbrown4d/orch/internal/dixdiag"
	"github.com/lyonbrown4d/orch/internal/raftsvc"
	orchruntime "github.com/lyonbrown4d/orch/internal/runtime"
)

// ReadyEndpoint serves GET /api/ready.
type ReadyEndpoint struct {
	cfg  config.Config
	raft *raftsvc.Service
	rt   *orchruntime.Manager
	diag *dixdiag.Service
}

func NewReadyEndpoint(cfg config.Config, raft *raftsvc.Service, rt *orchruntime.Manager, diag *dixdiag.Service) *ReadyEndpoint {
	return &ReadyEndpoint{cfg: cfg, raft: raft, rt: rt, diag: diag}
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

	raftReady, writeReady := e.addRaftReadiness(ctx, checks)
	runtimeReady := e.addRuntimeReadiness(checks)
	dixReady := e.addDixReadiness(ctx, checks)
	ready := raftReady && writeReady && runtimeReady && dixReady

	out := &ReadyOutput{}
	out.Body.Ready = ready
	out.Body.Status = readyStatus(ready)
	out.Body.Timestamp = time.Now().UTC().Format(time.RFC3339)
	out.Body.Checks = checks
	return out, nil
}

func (e *ReadyEndpoint) addRaftReadiness(ctx context.Context, checks *list.List[ReadyCheckItem]) (bool, bool) {
	if e == nil || e.raft == nil {
		checks.Add(ReadyCheckItem{Name: "raft", Ready: false, Status: "not_ready", Detail: "raft service unavailable"})
		checks.Add(ReadyCheckItem{Name: "writes", Ready: false, Status: "not_ready", Detail: "raft service unavailable"})
		return false, false
	}
	status, err := e.raft.Status(ctx)
	if err != nil {
		detail := err.Error()
		checks.Add(ReadyCheckItem{Name: "raft", Ready: false, Status: "error", Detail: detail})
		checks.Add(ReadyCheckItem{Name: "writes", Ready: false, Status: "error", Detail: detail})
		return false, false
	}
	raftReady := status.Ready && status.LeaderID != ""
	checks.Add(ReadyCheckItem{Name: "raft", Ready: raftReady, Status: status.State, Detail: status.Message})
	writeReady, writeDetail := e.writeReadiness(status)
	checks.Add(ReadyCheckItem{Name: "writes", Ready: writeReady, Status: readyStatus(writeReady), Detail: writeDetail})
	return raftReady, writeReady
}

func (e *ReadyEndpoint) writeReadiness(status raftsvc.Status) (bool, string) {
	if status.IsLeader {
		return true, ""
	}
	if status.LeaderID == "" {
		return false, "leader is not known"
	}
	if leaderURL, ok := e.cfg.Cluster.NodeURL(status.LeaderID); ok {
		return true, "writes forward to " + leaderURL
	}
	return false, "configure cluster.nodes." + status.LeaderID + " or target the leader API"
}

func (e *ReadyEndpoint) addRuntimeReadiness(checks *list.List[ReadyCheckItem]) bool {
	check := runtimeReadiness(e.rt)
	checks.Add(check)
	return check.Ready
}

func (e *ReadyEndpoint) addDixReadiness(ctx context.Context, checks *list.List[ReadyCheckItem]) bool {
	if e == nil || e.diag == nil {
		return true
	}
	report := e.diag.CheckReadiness(ctx)
	addDixHealthItems(checks, "dix/", report)
	return report.Healthy()
}

func readyStatus(ready bool) string {
	if ready {
		return "ready"
	}
	return "not_ready"
}
