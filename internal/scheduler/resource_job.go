package scheduler

import (
	"context"
	"strings"
	"time"

	gocron "github.com/go-co-op/gocron/v2"

	"github.com/daiyuang/orch/internal/nodecapacity"
	"github.com/daiyuang/orch/pkg/oopsx"
)

// RegisterResourceSnapshotJob runs an initial local refresh, then registers the periodic
// node resource snapshot job on s using cat.
func RegisterResourceSnapshotJob(ctx context.Context, s *Service, cat *nodecapacity.Catalog) error {
	if err := cat.RefreshLocal(ctx, s.cfg); err != nil {
		s.logger.Warn("initial node resource snapshot failed", "error", err)
	}
	sc := s.cfg.Scheduler
	resInt := 30 * time.Second
	if d, err := time.ParseDuration(strings.TrimSpace(sc.ResourceRefreshInterval)); err == nil && d > 0 {
		resInt = d
	}
	if _, err := s.Jobs().NewJob(
		gocron.DurationJob(resInt),
		gocron.NewTask(func() {
			cctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancel()
			if err := cat.RefreshLocal(cctx, s.cfg); err != nil {
				s.logger.Warn("node resource snapshot refresh failed", "error", err)
			}
		}),
		gocron.WithName("orch-node-resources"),
		gocron.WithTags("orch", "nodecapacity"),
		gocron.WithSingletonMode(gocron.LimitModeReschedule),
	); err != nil {
		return oopsx.B("scheduler").Wrapf(err, "register node resource refresh job")
	}
	return nil
}
