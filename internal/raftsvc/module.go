package raftsvc

import (
	"context"
	"log/slog"

	"github.com/arcgolabs/dix"
)

func Module() dix.Module {
	return dix.NewModule(
		"raft",
		dix.Providers(
			dix.Provider3(New),
		),
		dix.Hooks(
			dix.OnStart2(func(ctx context.Context, logger *slog.Logger, s *Service) error {
				logger.Info("lifecycle", "phase", "starting", "component", "raft")
				if err := s.Start(ctx); err != nil {
					logger.Error("lifecycle", "phase", "start_failed", "component", "raft", "error", err)
					return err
				}
				logger.Info("lifecycle", "phase", "started", "component", "raft")
				return nil
			}),
			dix.OnStop2(func(ctx context.Context, logger *slog.Logger, s *Service) error {
				logger.Info("lifecycle", "phase", "stopping", "component", "raft")
				if err := s.Stop(ctx); err != nil {
					logger.Warn("lifecycle", "phase", "stop_failed", "component", "raft", "error", err)
					return err
				}
				logger.Info("lifecycle", "phase", "stopped", "component", "raft")
				return nil
			}),
		),
	)
}
