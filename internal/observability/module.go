package observability

import (
	"context"
	"log/slog"

	"github.com/arcgolabs/dix"

	"github.com/lyonbrown4d/orch/internal/lifecycleplan"
)

func Module() dix.Module {
	return dix.NewModule(
		"observability",
		dix.Providers(
			dix.Provider1(NewPrometheusRegistry, dix.Eager()),
			dix.ProviderErr3(New, dix.Eager()),
		),
		dix.Hooks(
			dix.OnStop2(func(ctx context.Context, logger *slog.Logger, s *Service) error {
				if err := s.Shutdown(ctx); err != nil {
					logger.Warn("observability otlp shutdown incomplete", "error", err)
				}
				return nil
			}, dix.LifecycleName(lifecycleplan.HookObservability), dix.LifecyclePriority(lifecycleplan.PriorityShutdown), dix.LifecycleTimeout(lifecycleplan.TimeoutShutdown)),
		),
	)
}
