package gossipsvc

import (
	"context"
	"log/slog"

	"github.com/arcgolabs/dix"

	"github.com/lyonbrown4d/orch/internal/lifecycleplan"
)

func Module() dix.Module {
	return dix.NewModule(
		"gossip",
		dix.Providers(
			dix.Provider4(New, dix.Eager()),
		),
		dix.Hooks(
			dix.OnStart2(func(ctx context.Context, logger *slog.Logger, s *Service) error {
				logger.Info("lifecycle", "phase", "starting", "component", "gossip")
				if err := s.Start(ctx); err != nil {
					logger.Error("lifecycle", "phase", "start_failed", "component", "gossip", "error", err)
					return err
				}
				logger.Info("lifecycle", "phase", "started", "component", "gossip")
				return nil
			}, dix.LifecycleName(lifecycleplan.HookGossip), dix.LifecyclePriority(lifecycleplan.PriorityGossip), dix.LifecycleTimeout(lifecycleplan.TimeoutStart)),
			dix.OnStop2(func(ctx context.Context, logger *slog.Logger, s *Service) error {
				logger.Info("lifecycle", "phase", "stopping", "component", "gossip")
				if err := s.Stop(ctx); err != nil {
					logger.Warn("lifecycle", "phase", "stop_failed", "component", "gossip", "error", err)
					return err
				}
				logger.Info("lifecycle", "phase", "stopped", "component", "gossip")
				return nil
			}, dix.LifecycleName(lifecycleplan.HookGossip), dix.LifecyclePriority(lifecycleplan.PriorityGossip), dix.LifecycleTimeout(lifecycleplan.TimeoutStop)),
		),
	)
}
