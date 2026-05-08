package api

import (
	"context"

	"github.com/arcgolabs/collectionx/list"
	"github.com/arcgolabs/httpx"

	"github.com/daiyuang/orch/internal/raftsvc"
	"github.com/daiyuang/orch/pkg/oopsx"
)

type RaftStatusEndpoint struct {
	raft             *raftsvc.Service
	openAPIAuthApply bool
}

func NewRaftStatusEndpoint(raft *raftsvc.Service, openAPIAuthApply bool) *RaftStatusEndpoint {
	return &RaftStatusEndpoint{raft: raft, openAPIAuthApply: openAPIAuthApply}
}

func (e *RaftStatusEndpoint) EndpointSpec() httpx.EndpointSpec {
	spec := httpx.EndpointSpec{
		Prefix:      "/v1/raft/status",
		Description: "Inspect local Raft status and leader identity.",
		Tags:        httpx.Tags("raft"),
	}
	if e.openAPIAuthApply {
		spec.Security = httpx.SecurityRequirements(httpx.SecurityRequirement("bearerAuth"))
	}
	return spec
}

func (e *RaftStatusEndpoint) Register(r httpx.Registrar) {
	httpx.MustGroupGet(r.Scope(), "", e.status, OpenAPIMeta([]string{"raft"}, "getRaftStatus",
		"Get Raft status",
		"Returns the local Raft state, known leader, and current member configuration."))
}

func (e *RaftStatusEndpoint) status(ctx context.Context, _ *EmptyInput) (*RaftStatusOutput, error) {
	status, err := e.raft.Status(ctx)
	if err != nil {
		return nil, oopsx.B("api").Wrapf(err, "get raft status")
	}
	out := &RaftStatusOutput{}
	out.Body.Enabled = status.Enabled
	out.Body.Ready = status.Ready
	out.Body.NodeID = status.NodeID
	out.Body.State = status.State
	out.Body.IsLeader = status.IsLeader
	out.Body.LeaderID = status.LeaderID
	out.Body.LeaderAddress = status.LeaderAddress
	out.Body.LocalAddress = status.LocalAddress
	out.Body.Members = list.MapList(status.Members, func(_ int, member raftsvc.Member) RaftMemberItem {
		return raftMemberItem(member)
	})
	return out, nil
}
