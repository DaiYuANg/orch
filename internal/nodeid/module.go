package nodeid

import (
	"context"
	"log/slog"

	"github.com/arcgolabs/dix"
)

func Module() dix.Module {
	return dix.NewModule(
		"nodeid",
		dix.Providers(
			dix.ProviderErr1(New),
		),
		dix.Hooks(
			dix.OnStart2(func(ctx context.Context, logger *slog.Logger, local Local) error {
				_ = ctx
				logger.Info("runtime identity", "node_id", local.String())
				return nil
			}),
		),
	)
}
