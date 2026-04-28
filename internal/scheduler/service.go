package scheduler

import (
	"context"
	"log/slog"
	"time"

	gocron "github.com/go-co-op/gocron/v2"

	"github.com/daiyuang/orch/internal/config"
)

// Service wraps a gocron scheduler for periodic tasks.
type Service struct {
	logger *slog.Logger
	cfg    config.SchedulerConfig
	sched  gocron.Scheduler
}

// New constructs the scheduler worker (not started until Start).
func New(cfg config.Config, logger *slog.Logger) (*Service, error) {
	s, err := gocron.NewScheduler(gocron.WithLocation(time.Local))
	if err != nil {
		return nil, err
	}
	return &Service{
		logger: logger,
		cfg:    cfg.Scheduler,
		sched:  s,
	}, nil
}

// Start registers jobs and runs the scheduler (non-blocking).
func (s *Service) Start(_ context.Context) error {
	if !s.cfg.Enabled {
		s.logger.Info("scheduler disabled by config")
		return nil
	}

	interval := 2 * time.Minute
	if d, err := time.ParseDuration(s.cfg.HeartbeatInterval); err == nil && d > 0 {
		interval = d
	}

	if _, err := s.sched.NewJob(
		gocron.DurationJob(interval),
		gocron.NewTask(func() {
			s.logger.Debug("scheduler heartbeat")
		}),
		gocron.WithName("orch-heartbeat"),
		gocron.WithTags("orch", "heartbeat"),
	); err != nil {
		return err
	}

	s.sched.Start()
	s.logger.Info("scheduler started", "heartbeat_interval", interval.String())
	return nil
}

// Stop shuts down the scheduler; it cannot be restarted afterward.
func (s *Service) Stop(ctx context.Context) error {
	if !s.cfg.Enabled {
		return nil
	}
	if err := s.sched.ShutdownWithContext(ctx); err != nil {
		return err
	}
	s.logger.Info("scheduler stopped")
	return nil
}
