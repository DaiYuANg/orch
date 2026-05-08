package raftsvc

import (
	"context"
	"sort"
	"strings"
	"time"

	"github.com/arcgolabs/collectionx/list"
	hraft "github.com/hashicorp/raft"

	"github.com/daiyuang/orch/pkg/oopsx"
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
	if !s.cfg.Raft.Enabled || s.r == nil {
		return list.NewList[Member](), nil
	}
	future := s.r.GetConfiguration()
	if err := future.Error(); err != nil {
		return nil, oopsx.B("raft").Wrapf(err, "get raft configuration")
	}
	servers := future.Configuration().Servers
	sort.Slice(servers, func(i, j int) bool {
		return strings.Compare(string(servers[i].ID), string(servers[j].ID)) < 0
	})
	out := list.NewListWithCapacity[Member](len(servers))
	for _, server := range servers {
		out.Add(Member{
			ID:       string(server.ID),
			Address:  string(server.Address),
			Suffrage: server.Suffrage.String(),
		})
	}
	return out, nil
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
	if err := s.r.AddVoter(hraft.ServerID(id), hraft.ServerAddress(addr), 0, 30*time.Second).Error(); err != nil {
		return oopsx.B("raft").Wrapf(err, "add voter %q", id)
	}
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
	if err := s.r.RemoveServer(hraft.ServerID(id), 0, 30*time.Second).Error(); err != nil {
		return oopsx.B("raft").Wrapf(err, "remove server %q", id)
	}
	return nil
}

func (s *Service) ensureMembershipLeader() error {
	if s == nil {
		return oopsx.B("raft").Errorf("nil service")
	}
	if !s.cfg.Raft.Enabled || s.r == nil {
		return oopsx.B("raft").Errorf("raft is not ready")
	}
	if s.r.State() != hraft.Leader {
		return oopsx.B("raft").Errorf("not leader: send raft membership operation to the raft leader node")
	}
	return nil
}
