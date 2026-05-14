package raftsvc

import (
	"context"
	"sort"
	"strings"
	"time"

	"github.com/arcgolabs/collectionx/list"

	"github.com/lyonbrown4d/orch/pkg/oopsx"
)

type Member struct {
	ID       string `json:"id"`
	Address  string `json:"address"`
	Suffrage string `json:"suffrage"`
}

func (s *Service) ListMembers(ctx context.Context) (*list.List[Member], error) {
	if err := ctx.Err(); err != nil {
		return nil, oopsx.B("raft").Wrapf(err, "list members context")
	}
	if s == nil {
		return nil, oopsx.B("raft").Errorf("nil service")
	}
	if s.nh == nil {
		return list.NewList[Member](), nil
	}
	queryCtx, cancel := withDefaultDeadline(ctx, 5*time.Second)
	defer cancel()
	membership, err := s.nh.SyncGetShardMembership(queryCtx, controlShardID)
	if err != nil {
		return nil, oopsx.B("raft").Wrapf(err, "get raft membership")
	}
	replicaIDs := s.sortedMembershipReplicaIDs(membership.Nodes, membership.NonVotings, membership.Witnesses)
	out := list.NewListWithCapacity[Member](len(replicaIDs))
	for _, replicaID := range replicaIDs {
		out.Add(s.memberFromReplicaID(replicaID, membership.Nodes, membership.NonVotings, membership.Witnesses))
	}
	return out, nil
}

func (s *Service) sortedMembershipReplicaIDs(nodes, nonVotings, witnesses map[uint64]string) []uint64 {
	replicaIDs := make([]uint64, 0, len(nodes)+len(nonVotings)+len(witnesses))
	replicaIDs = appendReplicaIDs(replicaIDs, nodes)
	replicaIDs = appendReplicaIDs(replicaIDs, nonVotings)
	replicaIDs = appendReplicaIDs(replicaIDs, witnesses)
	sort.Slice(replicaIDs, func(i, j int) bool {
		return s.nodeIDForMember(replicaIDs[i], "") < s.nodeIDForMember(replicaIDs[j], "")
	})
	return replicaIDs
}

func appendReplicaIDs(replicaIDs []uint64, targets map[uint64]string) []uint64 {
	for replicaID := range targets {
		replicaIDs = append(replicaIDs, replicaID)
	}
	return replicaIDs
}

func (s *Service) memberFromReplicaID(replicaID uint64, nodes, nonVotings, witnesses map[uint64]string) Member {
	address, suffrage := memberAddressAndSuffrage(replicaID, nodes, nonVotings, witnesses)
	return Member{
		ID:       s.nodeIDForMember(replicaID, address),
		Address:  address,
		Suffrage: suffrage,
	}
}

func memberAddressAndSuffrage(replicaID uint64, nodes, nonVotings, witnesses map[uint64]string) (string, string) {
	if address, ok := nonVotings[replicaID]; ok {
		return address, "NonVoter"
	}
	if address, ok := witnesses[replicaID]; ok {
		return address, "Witness"
	}
	return nodes[replicaID], "Voter"
}

func (s *Service) AddVoter(ctx context.Context, id, address string) error {
	if err := ctx.Err(); err != nil {
		return oopsx.B("raft").Wrapf(err, "add voter context")
	}
	if err := s.ensureMembershipLeader(); err != nil {
		return err
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return oopsx.B("raft").Errorf("member id is required")
	}
	addr, err := validateRaftPeerAddress("member.address", address)
	if err != nil {
		return oopsx.B("raft").Wrapf(err, "validate member address")
	}
	replicaID, err := replicaIDForNodeID(id)
	if err != nil {
		return err
	}
	queryCtx, cancel := withDefaultDeadline(ctx, 30*time.Second)
	defer cancel()
	if err := s.nh.SyncRequestAddReplica(queryCtx, controlShardID, replicaID, addr, 0); err != nil {
		return oopsx.B("raft").Wrapf(err, "add voter %q", id)
	}
	s.rememberMember(id, replicaID, addr)
	return nil
}

func (s *Service) RemoveServer(ctx context.Context, id string) error {
	if err := ctx.Err(); err != nil {
		return oopsx.B("raft").Wrapf(err, "remove server context")
	}
	if err := s.ensureMembershipLeader(); err != nil {
		return err
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return oopsx.B("raft").Errorf("member id is required")
	}
	replicaID, err := replicaIDForNodeID(id)
	if err != nil {
		return err
	}
	queryCtx, cancel := withDefaultDeadline(ctx, 30*time.Second)
	defer cancel()
	if err := s.nh.SyncRequestDeleteReplica(queryCtx, controlShardID, replicaID, 0); err != nil {
		return oopsx.B("raft").Wrapf(err, "remove server %q", id)
	}
	s.forgetMember(replicaID)
	return nil
}

func (s *Service) ensureMembershipLeader() error {
	if s == nil {
		return oopsx.B("raft").Errorf("nil service")
	}
	if s.nh == nil {
		return oopsx.B("raft").Errorf("raft is not ready")
	}
	if !s.isLocalLeader() {
		return oopsx.B("raft").Errorf("not leader: send raft membership operation to the raft leader node")
	}
	return nil
}
