package raftsvc

import (
	"context"

	"github.com/arcgolabs/collectionx/list"

	"github.com/daiyuang/orch/pkg/oopsx"
)

type Status struct {
	Ready         bool
	NodeID        string
	State         string
	IsLeader      bool
	LeaderID      string
	LeaderAddress string
	LocalAddress  string
	Message       string
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
		NodeID:  s.localID.String(),
		Members: list.NewList[Member](),
	}
	if s.nh == nil {
		return notReadyStatus(status), nil
	}

	localAddress := s.localAddress
	leaderReplicaID, leaderReady, leaderMessage := s.leaderSnapshot()
	if leaderMessage != "" {
		return unknownLeaderStatus(status, localAddress, leaderMessage), nil
	}
	isLeader := leaderReady && leaderReplicaID == s.localReplicaID
	members, err := s.ListMembers(ctx)
	if err != nil {
		members = list.NewList[Member]()
	}
	leaderAddress := s.leaderAddress(leaderReplicaID, isLeader, localAddress, members)

	status.Ready = leaderReady
	status.State = raftStateName(leaderReady, isLeader)
	status.IsLeader = isLeader
	if leaderReady {
		status.LeaderID = s.nodeIDForMember(leaderReplicaID, leaderAddress)
	} else {
		status.Message = "raft leader is not ready"
	}
	status.LeaderAddress = leaderAddress
	status.LocalAddress = localAddress
	status.Members = members
	return status, nil
}

func notReadyStatus(status Status) Status {
	status.State = "not_ready"
	status.Message = "dragonboat nodehost is not started"
	return status
}

func unknownLeaderStatus(status Status, localAddress, message string) Status {
	status.State = "unknown"
	status.LocalAddress = localAddress
	status.Message = message
	return status
}

func raftStateName(leaderReady, isLeader bool) string {
	if isLeader {
		return "Leader"
	}
	if !leaderReady {
		return "Unknown"
	}
	return "Follower"
}

func (s *Service) leaderAddress(leaderReplicaID uint64, isLeader bool, localAddress string, members *list.List[Member]) string {
	address := ""
	members.Range(func(_ int, member Member) bool {
		if member.ID == s.nodeIDForMember(leaderReplicaID, member.Address) {
			address = member.Address
			return false
		}
		return true
	})
	if isLeader && address == "" {
		return localAddress
	}
	return address
}

func (s *Service) leaderSnapshot() (uint64, bool, string) {
	leaderReplicaID, _, leaderReady, err := s.nh.GetLeaderID(controlShardID)
	if err != nil {
		return 0, false, err.Error()
	}
	return leaderReplicaID, leaderReady, ""
}
