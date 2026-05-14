package scheduler

import (
	"context"

	gocron "github.com/go-co-op/gocron/v2"

	"github.com/lyonbrown4d/orch/internal/raftsvc"
	"github.com/lyonbrown4d/orch/pkg/oopsx"
)

var _ gocron.Elector = (*raftElector)(nil)

type raftElector struct {
	raft *raftsvc.Service
}

func newRaftElector(r *raftsvc.Service) *raftElector {
	return &raftElector{raft: r}
}

func (e *raftElector) IsLeader(ctx context.Context) error {
	if err := e.raft.SchedulerLeadership(ctx); err != nil {
		return oopsx.B("scheduler").Wrapf(err, "raft leadership")
	}
	return nil
}
