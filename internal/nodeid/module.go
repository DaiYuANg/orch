package nodeid

import (
	"context"
	"log/slog"

	"github.com/arcgolabs/dix"

	"github.com/daiyuang/orch/internal/lifecycleplan"
)

func Module() dix.Module {
	return dix.NewModule(
		"nodeid",
		dix.Providers(
			dix.ProviderErr1(New, dix.Eager()),
		),
		dix.Hooks(
			dix.OnStart2(func(ctx context.Context, logger *slog.Logger, local Local) error {
				_ = ctx
				logger.Info("runtime identity", "node_id", local.String())
				return nil
			}, dix.LifecycleName(lifecycleplan.HookNodeID), dix.LifecyclePriority(lifecycleplan.PriorityIdentity), dix.LifecycleTimeout(lifecycleplan.TimeoutShort)),
		),
	)
}
