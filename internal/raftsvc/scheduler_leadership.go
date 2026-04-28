package raftsvc

import (
	"context"
	"fmt"

	hraft "github.com/hashicorp/raft"
)

// SchedulerLeadership backs gocron's Elector (WithDistributedElector).
// When Raft is disabled, every instance may run scheduled jobs (single-process / non-HA deployments).
// When Raft is enabled, only the cluster leader runs jobs unless scheduler config disables leader-only mode.
func (s *Service) SchedulerLeadership(ctx context.Context) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if !s.cfg.Raft.Enabled {
		return nil
	}
	if s.r == nil {
		return fmt.Errorf("orch raft: not ready")
	}
	if s.r.State() != hraft.Leader {
		return fmt.Errorf("orch raft: not leader")
	}
	return nil
}
