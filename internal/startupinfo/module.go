package startupinfo

import (
	"context"
	"log/slog"

	"github.com/arcgolabs/dix"

	"github.com/daiyuang/orch/internal/config"
	"github.com/daiyuang/orch/internal/lifecycleplan"
)

// Module registers a lifecycle hook that emits one consolidated reachability log after every
// other module has bound and run its OnStart hooks. Register this module last in dix.Modules.
func Module() dix.Module {
	return dix.NewModule(
		"startupinfo",
		dix.Hooks(
			dix.OnStart2(func(ctx context.Context, logger *slog.Logger, cfg config.Config) error {
				_ = ctx
				config.LogReachableEndpoints(logger, cfg)
				return nil
			}, dix.LifecycleName(lifecycleplan.HookStartupInfo), dix.LifecyclePriority(lifecycleplan.PriorityReady), dix.LifecycleTimeout(lifecycleplan.TimeoutReady)),
		),
	)
}
