package scheduler

import (
	"time"

	gocron "github.com/go-co-op/gocron/v2"

	"github.com/daiyuang/orch/pkg/oopsx"
)

// RegisterHeartbeatJob registers the periodic debug heartbeat task on s.
func RegisterHeartbeatJob(s *Service) error {
	sc := s.cfg.Scheduler
	interval := 2 * time.Minute
	if d, err := time.ParseDuration(sc.HeartbeatInterval); err == nil && d > 0 {
		interval = d
	}
	if _, err := s.Jobs().NewJob(
		gocron.DurationJob(interval),
		gocron.NewTask(func() {
			s.logger.Debug("scheduler job completed",
				"component", "scheduler",
				"job", "orch-heartbeat",
				"event", "tick",
			)
		}),
		gocron.WithName("orch-heartbeat"),
		gocron.WithTags("orch", "heartbeat"),
		gocron.WithSingletonMode(gocron.LimitModeReschedule),
	); err != nil {
		return oopsx.B("scheduler").Wrapf(err, "register heartbeat job")
	}
	return nil
}
