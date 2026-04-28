package logging

import (
	"context"
	"log/slog"

	"github.com/arcgolabs/dix"
	"github.com/arcgolabs/logx"

	"github.com/daiyuang/orch/internal/config"
)

func Module() dix.Module {
	return dix.NewModule(
		"logger",
		dix.Providers(
			dix.ProviderErr1(func(cfg config.Config) (*slog.Logger, error) {
				return New(cfg.Log)
			}),
		),
		dix.Hooks(
			dix.OnStop(func(_ context.Context, logger *slog.Logger) error {
				logger.Info("lifecycle", "phase", "closing_log_sink", "component", "logging")
				return logx.Close(logger)
			}),
		),
	)
}
