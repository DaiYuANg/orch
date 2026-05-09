package httpserver

import (
	"context"
	"log/slog"

	"github.com/arcgolabs/dix"

	"github.com/daiyuang/orch/internal/lifecycleplan"
)

func Module() dix.Module {
	return dix.NewModule(
		"http-server",
		dix.Providers(
			dix.ProviderErr4(New, dix.Eager()),
		),
		dix.Hooks(
			dix.OnStart2(func(ctx context.Context, logger *slog.Logger, s *Server) error {
				logger.Info("lifecycle", "phase", "starting", "component", "httpserver")
				if err := s.Start(context.WithoutCancel(ctx)); err != nil {
					logger.Error("lifecycle", "phase", "start_failed", "component", "httpserver", "error", err)
					return err
				}
				logger.Info("lifecycle", "phase", "started", "component", "httpserver")
				return nil
			}, dix.LifecycleName(lifecycleplan.HookHTTPServer), dix.LifecyclePriority(lifecycleplan.PriorityNetwork), dix.LifecycleParallel(), dix.LifecycleTimeout(lifecycleplan.TimeoutStart)),
			dix.OnStop2(func(ctx context.Context, logger *slog.Logger, s *Server) error {
				logger.Info("lifecycle", "phase", "stopping", "component", "httpserver")
				if err := s.Stop(ctx); err != nil {
					logger.Warn("lifecycle", "phase", "stop_failed", "component", "httpserver", "error", err)
					return err
				}
				logger.Info("lifecycle", "phase", "stopped", "component", "httpserver")
				return nil
			}, dix.LifecycleName(lifecycleplan.HookHTTPServer), dix.LifecyclePriority(lifecycleplan.PriorityNetwork), dix.LifecycleParallel(), dix.LifecycleTimeout(lifecycleplan.TimeoutStop)),
		),
	)
}
