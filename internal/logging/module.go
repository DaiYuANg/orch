package logging

import (
	"context"
	"log/slog"

	"github.com/arcgolabs/dix"
	"github.com/arcgolabs/logx"

	"github.com/lyonbrown4d/orch/internal/config"
	"github.com/lyonbrown4d/orch/internal/lifecycleplan"
)

func Module() dix.Module {
	return dix.NewModule(
		"logger",
		dix.Providers(
			dix.ProviderErr1(func(cfg config.Config) (*slog.Logger, error) {
				return New(cfg.Log)
			}, dix.Eager()),
		),
		dix.Hooks(
			dix.OnStart(func(_ context.Context, logger *slog.Logger) error {
				slog.SetDefault(logger)
				return nil
			}, dix.LifecycleName(lifecycleplan.HookLogging), dix.LifecyclePriority(lifecycleplan.PriorityLogging), dix.LifecycleTimeout(lifecycleplan.TimeoutShort)),
			dix.OnStop(func(_ context.Context, logger *slog.Logger) error {
				logger.Info("lifecycle", "phase", "closing_log_sink", "component", "logging")
				return logx.Close(logger)
			}, dix.LifecycleName(lifecycleplan.HookLogging), dix.LifecyclePriority(lifecycleplan.PriorityLogging), dix.LifecycleTimeout(lifecycleplan.TimeoutShutdown)),
		),
	)
}
