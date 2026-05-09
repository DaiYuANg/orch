package dnssvc

import (
	"context"
	"log/slog"

	"github.com/arcgolabs/dix"

	"github.com/daiyuang/orch/internal/lifecycleplan"
)

func Module() dix.Module {
	return dix.NewModule(
		"dns",
		dix.Providers(
			dix.Provider2(New, dix.Eager()),
		),
		dix.Hooks(
			dix.OnStart2(func(ctx context.Context, logger *slog.Logger, s *Service) error {
				logger.Info("lifecycle", "phase", "starting", "component", "dns")
				if err := s.Start(context.WithoutCancel(ctx)); err != nil {
					logger.Error("lifecycle", "phase", "start_failed", "component", "dns", "error", err)
					return err
				}
				logger.Info("lifecycle", "phase", "started", "component", "dns")
				return nil
			}, dix.LifecycleName(lifecycleplan.HookDNS), dix.LifecyclePriority(lifecycleplan.PriorityNetwork), dix.LifecycleParallel(), dix.LifecycleTimeout(lifecycleplan.TimeoutStart)),
			dix.OnStop2(func(ctx context.Context, logger *slog.Logger, s *Service) error {
				logger.Info("lifecycle", "phase", "stopping", "component", "dns")
				if err := s.Stop(ctx); err != nil {
					logger.Warn("lifecycle", "phase", "stop_failed", "component", "dns", "error", err)
					return err
				}
				logger.Info("lifecycle", "phase", "stopped", "component", "dns")
				return nil
			}, dix.LifecycleName(lifecycleplan.HookDNS), dix.LifecyclePriority(lifecycleplan.PriorityNetwork), dix.LifecycleParallel(), dix.LifecycleTimeout(lifecycleplan.TimeoutStop)),
		),
	)
}
