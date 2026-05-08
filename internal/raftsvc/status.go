package raftsvc

import (
	"context"

	"github.com/arcgolabs/collectionx/list"
	hraft "github.com/hashicorp/raft"

	"github.com/daiyuang/orch/pkg/oopsx"
)

type Status struct {
	Enabled       bool
	Ready         bool
	NodeID        string
	State         string
	IsLeader      bool
	LeaderID      string
	LeaderAddress string
	LocalAddress  string
	Members       *list.List[Member]
}

func (s *Service) Status(ctx context.Context) (Status, error) {
	if err := ctx.Err(); err != nil {
		return Status{}, oopsx.B("raft").Wrapf(err, "status context")
	}
	if s == nil {
		return Status{}, oopsx.B("raft").Errorf("nil service")
	}

	status := Status{
		Enabled: s.cfg.Raft.Enabled,
		NodeID:  s.localID.String(),
		Members: list.NewList[Member](),
	}
	if !s.cfg.Raft.Enabled {
		status.State = "disabled"
		return status, nil
	}
	if s.r == nil {
		status.State = "not_ready"
		return status, nil
	}

	localAddress := ""
	if s.transport != nil {
		localAddress = string(s.transport.LocalAddr())
	}
	state := s.r.State()
	leaderAddress, leaderID := s.r.LeaderWithID()
	if state == hraft.Leader {
		if leaderID == "" {
			leaderID = hraft.ServerID(status.NodeID)
		}
		if leaderAddress == "" {
			leaderAddress = hraft.ServerAddress(localAddress)
		}
	}

	members, err := s.ListMembers(ctx)
	if err != nil {
		return Status{}, err
	}
	status.Ready = true
	status.State = state.String()
	status.IsLeader = state == hraft.Leader
	status.LeaderID = string(leaderID)
	status.LeaderAddress = string(leaderAddress)
	status.LocalAddress = localAddress
	status.Members = members
	return status, nil
}
