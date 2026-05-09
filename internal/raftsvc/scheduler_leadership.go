package raftsvc

import (
	"context"

	"github.com/daiyuang/orch/pkg/oopsx"
)

// SchedulerLeadership backs gocron's Elector (WithDistributedElector).
// When Raft is disabled, every instance may run scheduled jobs (single-process / non-HA deployments).
// When Raft is enabled, only the cluster leader runs jobs unless scheduler config disables leader-only mode.
func (s *Service) SchedulerLeadership(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return oopsx.B("raft").Wrapf(err, "scheduler leadership context")
	}
	if !s.cfg.Raft.Enabled {
		return nil
	}
	if s.nh == nil {
		return oopsx.B("raft").Errorf("orch raft: not ready")
	}
	if !s.isLocalLeader() {
		return oopsx.B("raft").Errorf("orch raft: not leader")
	}
	return nil
}
