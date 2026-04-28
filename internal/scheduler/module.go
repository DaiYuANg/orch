package scheduler

import (
	"context"
	"log/slog"

	"github.com/arcgolabs/dix"
)

func Module() dix.Module {
	return dix.NewModule(
		"scheduler",
		dix.Providers(
			dix.ProviderErr2(New),
		),
		dix.Hooks(
			dix.OnStart2(func(ctx context.Context, logger *slog.Logger, s *Service) error {
				logger.Info("lifecycle", "phase", "starting", "component", "scheduler")
				if err := s.Start(ctx); err != nil {
					logger.Error("lifecycle", "phase", "start_failed", "component", "scheduler", "error", err)
					return err
				}
				logger.Info("lifecycle", "phase", "started", "component", "scheduler")
				return nil
			}),
			dix.OnStop2(func(ctx context.Context, logger *slog.Logger, s *Service) error {
				logger.Info("lifecycle", "phase", "stopping", "component", "scheduler")
				if err := s.Stop(ctx); err != nil {
					logger.Warn("lifecycle", "phase", "stop_failed", "component", "scheduler", "error", err)
					return err
				}
				logger.Info("lifecycle", "phase", "stopped", "component", "scheduler")
				return nil
			}),
		),
	)
}
