package gossipsvc

import (
	"context"
	"strings"
	"time"

	"github.com/arcgolabs/collectionx/list"

	"github.com/lyonbrown4d/orch/internal/raftsvc"
	"github.com/lyonbrown4d/orch/pkg/oopsx"
)

func (s *Service) reconcileLoop(ctx context.Context) {
	defer s.wg.Done()
	interval := s.reconcileInterval()
	s.reconcileOnce(ctx)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.reconcileOnce(ctx)
		}
	}
}

func (s *Service) reconcileInterval() time.Duration {
	interval, err := time.ParseDuration(strings.TrimSpace(s.cfg.Gossip.ReconcileInterval))
	if err != nil || interval <= 0 {
		return 5 * time.Second
	}
	return interval
}

func (s *Service) reconcileOnce(ctx context.Context) {
	if err := ctx.Err(); err != nil {
		return
	}
	s.joinSeeds(ctx)
	s.refreshMembers()
	if !s.cfg.Gossip.AutoJoinRaft {
		return
	}
	if err := s.reconcileRaftMembership(ctx); err != nil {
		s.logger.Warn("gossip raft auto join failed", "error", err)
	}
}

func (s *Service) joinSeeds(ctx context.Context) {
	if s == nil || s.ml == nil {
		return
	}
	if err := ctx.Err(); err != nil {
		return
	}
	seeds := normalizeSeeds(s.cfg.Gossip.Seeds)
	if seeds.Len() == 0 {
		return
	}
	joined, err := s.ml.Join(seeds.Values())
	if err != nil {
		s.logger.Warn("gossip seed join failed", "error", err, "seeds", seeds.Values())
		return
	}
	if joined > 0 {
		s.logger.Info("gossip seed join", "joined", joined)
	}
}

func (s *Service) reconcileRaftMembership(ctx context.Context) error {
	if s == nil || s.raft == nil {
		return nil
	}
	status, err := s.raft.Status(ctx)
	if err != nil {
		return oopsx.B("gossip", "raft").Wrapf(err, "read raft status")
	}
	if !status.Ready || !status.IsLeader {
		return nil
	}
	existing, err := raftMemberIDs(ctx, s.raft)
	if err != nil {
		return err
	}
	candidates := s.raftJoinCandidates(existing)
	var joinErr error
	candidates.Range(func(_ int, node Node) bool {
		if err := ctx.Err(); err != nil {
			joinErr = oopsx.B("gossip", "raft").Wrapf(err, "auto join context")
			return false
		}
		if err := s.raft.AddVoter(ctx, node.ID, node.RaftAddress); err != nil {
			joinErr = oopsx.B("gossip", "raft").Wrapf(err, "add discovered voter %s", node.ID)
			return false
		}
		existing[node.ID] = struct{}{}
		s.logger.Info("gossip discovered raft voter joined", "node_id", node.ID, "raft_address", node.RaftAddress)
		return true
	})
	return joinErr
}

func raftMemberIDs(ctx context.Context, raft *raftsvc.Service) (map[string]struct{}, error) {
	members, err := raft.ListMembers(ctx)
	if err != nil {
		return nil, oopsx.B("gossip", "raft").Wrapf(err, "list raft members")
	}
	out := make(map[string]struct{}, members.Len())
	members.Range(func(_ int, member raftsvc.Member) bool {
		if id := strings.TrimSpace(member.ID); id != "" {
			out[id] = struct{}{}
		}
		return true
	})
	return out, nil
}

func (s *Service) raftJoinCandidates(existing map[string]struct{}) *list.List[Node] {
	localID := strings.TrimSpace(s.local.String())
	out := list.NewList[Node]()
	s.members.Range(func(_ string, node Node) bool {
		if !isRaftJoinCandidate(node, existing, localID) {
			return true
		}
		out.Add(node)
		return true
	})
	out.Sort(func(a, b Node) int {
		return strings.Compare(a.ID, b.ID)
	})
	return out
}

func isRaftJoinCandidate(node Node, existing map[string]struct{}, localID string) bool {
	id := strings.TrimSpace(node.ID)
	if id == "" || id == localID || strings.TrimSpace(node.RaftAddress) == "" {
		return false
	}
	if !strings.EqualFold(strings.TrimSpace(node.State), "alive") {
		return false
	}
	_, ok := existing[id]
	return !ok
}
