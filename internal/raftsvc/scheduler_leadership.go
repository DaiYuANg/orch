package raftsvc

import (
	"context"

	"github.com/daiyuang/orch/pkg/oopsx"
)

// SchedulerLeadership backs gocron's Elector (WithDistributedElector).
// Only the cluster leader runs jobs unless scheduler config disables leader-only mode.
func (s *Service) SchedulerLeadership(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return oopsx.B("raft").Wrapf(err, "scheduler leadership context")
	}
	if s.nh == nil {
		return oopsx.B("raft").Errorf("orch raft: not ready")
	}
	if !s.isLocalLeader() {
		return oopsx.B("raft").Errorf("orch raft: not leader")
	}
	return nil
}
