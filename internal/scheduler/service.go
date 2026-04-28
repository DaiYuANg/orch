package scheduler

import (
	"context"
	"log/slog"
	"strings"
	"time"

	gocron "github.com/go-co-op/gocron/v2"

	"github.com/daiyuang/orch/internal/config"
	"github.com/daiyuang/orch/internal/raftsvc"
	"github.com/daiyuang/orch/pkg/oopsx"
)

// Service wraps a gocron scheduler for periodic tasks.
type Service struct {
	logger *slog.Logger
	cfg    config.SchedulerConfig
	sched  gocron.Scheduler
}

// New constructs the scheduler worker (not started until Start).
func New(cfg config.Config, logger *slog.Logger, raft *raftsvc.Service) (*Service, error) {
	opts := []gocron.SchedulerOption{
		gocron.WithLocation(time.Local),
	}

	if cfg.Raft.Enabled && cfg.Scheduler.RaftLeaderOnly {
		opts = append(opts, gocron.WithDistributedElector(newRaftElector(raft)))
	}

	if cfg.Scheduler.MaxConcurrentJobs > 0 {
		var mode gocron.LimitMode = gocron.LimitModeReschedule
		if strings.EqualFold(strings.TrimSpace(cfg.Scheduler.ConcurrentJobsMode), "wait") {
			mode = gocron.LimitModeWait
		}
		opts = append(opts, gocron.WithLimitConcurrentJobs(cfg.Scheduler.MaxConcurrentJobs, mode))
	}

	s, err := gocron.NewScheduler(opts...)
	if err != nil {
		return nil, oopsx.B("scheduler").Wrapf(err, "new gocron scheduler")
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
		gocron.WithSingletonMode(gocron.LimitModeReschedule),
	); err != nil {
		return oopsx.B("scheduler").Wrapf(err, "register heartbeat job")
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
		return oopsx.B("scheduler").Wrapf(err, "shutdown scheduler")
	}
	s.logger.Info("scheduler stopped")
	return nil
}
