package raftsvc

import (
	"context"

	"github.com/arcgolabs/collectionx/list"

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
	if s.nh == nil {
		status.State = "not_ready"
		return status, nil
	}

	localAddress := s.localAddress
	leaderReplicaID, _, leaderReady, err := s.nh.GetLeaderID(controlShardID)
	if err != nil {
		status.Ready = true
		status.State = "unknown"
		status.LocalAddress = localAddress
		return status, nil
	}
	isLeader := leaderReady && leaderReplicaID == s.localReplicaID
	state := "Follower"
	if isLeader {
		state = "Leader"
	} else if !leaderReady {
		state = "Unknown"
	}

	members, err := s.ListMembers(ctx)
	if err != nil {
		members = list.NewList[Member]()
	}
	leaderAddress := ""
	members.Range(func(_ int, member Member) bool {
		if member.ID == s.nodeIDForMember(leaderReplicaID, member.Address) {
			leaderAddress = member.Address
			return false
		}
		return true
	})
	if isLeader && leaderAddress == "" {
		leaderAddress = localAddress
	}

	status.Ready = true
	status.State = state
	status.IsLeader = isLeader
	if leaderReady {
		status.LeaderID = s.nodeIDForMember(leaderReplicaID, leaderAddress)
	}
	status.LeaderAddress = leaderAddress
	status.LocalAddress = localAddress
	status.Members = members
	return status, nil
}
