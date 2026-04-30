package observability

import (
	"context"
	"log/slog"

	"github.com/arcgolabs/dix"
)

func Module() dix.Module {
	return dix.NewModule(
		"observability",
		dix.Providers(
			dix.Provider1(NewPrometheusRegistry),
			dix.ProviderErr3(New),
		),
		dix.Hooks(
			dix.OnStop2(func(ctx context.Context, logger *slog.Logger, s *Service) error {
				if err := s.Shutdown(ctx); err != nil {
					logger.Warn("observability otlp shutdown incomplete", "error", err)
				}
				return nil
			}),
		),
	)
}
