package api

import (
	"context"
	"strings"

	"github.com/arcgolabs/collectionx/list"
	"github.com/arcgolabs/httpx"

	"github.com/daiyuang/orch/internal/raftsvc"
	"github.com/daiyuang/orch/pkg/oopsx"
)

type RaftMembersEndpoint struct {
	raft             *raftsvc.Service
	openAPIAuthApply bool
}

func NewRaftMembersEndpoint(raft *raftsvc.Service, openAPIAuthApply bool) *RaftMembersEndpoint {
	return &RaftMembersEndpoint{raft: raft, openAPIAuthApply: openAPIAuthApply}
}

func (e *RaftMembersEndpoint) EndpointSpec() httpx.EndpointSpec {
	spec := httpx.EndpointSpec{
		Prefix:      "/v1/raft/members",
		Description: "Inspect and update Raft cluster membership.",
		Tags:        httpx.Tags("raft"),
	}
	if e.openAPIAuthApply {
		spec.Security = httpx.SecurityRequirements(httpx.SecurityRequirement("bearerAuth"))
	}
	return spec
}

func (e *RaftMembersEndpoint) Register(r httpx.Registrar) {
	httpx.MustGroupGet(r.Scope(), "", e.list, OpenAPIMeta([]string{"raft"}, "listRaftMembers",
		"List Raft members",
		"Returns the current Raft voter/non-voter configuration from this node."))
	httpx.MustGroupPost(r.Scope(), "", e.add, OpenAPIMeta([]string{"raft"}, "addRaftVoter",
		"Add Raft voter",
		"Adds or updates a Raft voter. Target the current Raft leader."))
	httpx.MustGroupDelete(r.Scope(), "/{id}", e.remove, OpenAPIMeta([]string{"raft"}, "removeRaftMember",
		"Remove Raft member",
		"Removes a Raft server from membership. Target the current Raft leader."))
}

func (e *RaftMembersEndpoint) list(ctx context.Context, _ *EmptyInput) (*ListRaftMembersOutput, error) {
	members, err := e.raft.ListMembers(ctx)
	if err != nil {
		return nil, oopsx.B("api").Wrapf(err, "list raft members")
	}
	out := &ListRaftMembersOutput{}
	out.Body.Items = list.MapList(members, func(_ int, member raftsvc.Member) RaftMemberItem {
		return raftMemberItem(member)
	})
	return out, nil
}

func (e *RaftMembersEndpoint) add(ctx context.Context, in *AddRaftMemberInput) (*AddRaftMemberOutput, error) {
	id := strings.TrimSpace(in.Body.ID)
	address := strings.TrimSpace(in.Body.Address)
	if err := e.raft.AddVoter(ctx, id, address); err != nil {
		return nil, oopsx.B("api").Wrapf(err, "add raft voter")
	}
	out := &AddRaftMemberOutput{}
	out.Body.Accepted = true
	out.Body.Member = RaftMemberItem{ID: id, Address: address, Suffrage: "Voter"}
	return out, nil
}

func (e *RaftMembersEndpoint) remove(ctx context.Context, in *RemoveRaftMemberInput) (*RemoveRaftMemberOutput, error) {
	id := strings.TrimSpace(in.ID)
	if err := e.raft.RemoveServer(ctx, id); err != nil {
		return nil, oopsx.B("api").Wrapf(err, "remove raft member")
	}
	out := &RemoveRaftMemberOutput{}
	out.Body.Accepted = true
	out.Body.ID = id
	return out, nil
}

func raftMemberItem(member raftsvc.Member) RaftMemberItem {
	return RaftMemberItem{
		ID:       member.ID,
		Address:  member.Address,
		Suffrage: member.Suffrage,
	}
}
