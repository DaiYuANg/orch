package ingress

import (
	"context"
	"log/slog"

	"github.com/arcgolabs/dix"
)

// Module wires embedded Caddy lifecycle. Registration order in cmd/orch-server must place
// this module after dns and before scheduler so start order remains: raft → dns → ingress → …
func Module() dix.Module {
	return dix.NewModule(
		"ingress",
		dix.Providers(
			dix.Provider2(New),
		),
		dix.Hooks(
			dix.OnStart2(func(ctx context.Context, logger *slog.Logger, s *Service) error {
				logger.Info("lifecycle", "phase", "starting", "component", "ingress")
				if err := s.Start(ctx); err != nil {
					logger.Error("lifecycle", "phase", "start_failed", "component", "ingress", "error", err)
					return err
				}
				logger.Info("lifecycle", "phase", "started", "component", "ingress")
				return nil
			}),
			dix.OnStop2(func(ctx context.Context, logger *slog.Logger, s *Service) error {
				logger.Info("lifecycle", "phase", "stopping", "component", "ingress")
				if err := s.Stop(ctx); err != nil {
					logger.Warn("lifecycle", "phase", "stop_failed", "component", "ingress", "error", err)
					return err
				}
				logger.Info("lifecycle", "phase", "stopped", "component", "ingress")
				return nil
			}),
		),
	)
}
