package scheduler

import (
	"context"

	gocron "github.com/go-co-op/gocron/v2"

	"github.com/daiyuang/orch/internal/raftsvc"
)

var _ gocron.Elector = (*raftElector)(nil)

type raftElector struct {
	raft *raftsvc.Service
}

func newRaftElector(r *raftsvc.Service) *raftElector {
	return &raftElector{raft: r}
}

func (e *raftElector) IsLeader(ctx context.Context) error {
	return e.raft.SchedulerLeadership(ctx)
}
