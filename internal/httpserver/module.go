package httpserver

import (
	"context"
	"log/slog"

	"github.com/arcgolabs/dix"
)

func Module() dix.Module {
	return dix.NewModule(
		"http-server",
		dix.Providers(
			dix.ProviderErr4(New),
		),
		dix.Hooks(
			dix.OnStart2(func(ctx context.Context, logger *slog.Logger, s *Server) error {
				logger.Info("lifecycle", "phase", "starting", "component", "httpserver")
				if err := s.Start(ctx); err != nil {
					logger.Error("lifecycle", "phase", "start_failed", "component", "httpserver", "error", err)
					return err
				}
				logger.Info("lifecycle", "phase", "started", "component", "httpserver")
				return nil
			}),
			dix.OnStop2(func(ctx context.Context, logger *slog.Logger, s *Server) error {
				logger.Info("lifecycle", "phase", "stopping", "component", "httpserver")
				if err := s.Stop(ctx); err != nil {
					logger.Warn("lifecycle", "phase", "stop_failed", "component", "httpserver", "error", err)
					return err
				}
				logger.Info("lifecycle", "phase", "stopped", "component", "httpserver")
				return nil
			}),
		),
	)
}
