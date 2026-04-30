package orchlsp

import (
	"context"
	"log/slog"

	"github.com/arcgolabs/dix"
	"github.com/arcgolabs/plano/compiler"
	plsp "github.com/arcgolabs/plano/lsp"
)

// Module registers a start hook that runs [plsp.ServeStdio] with the process singleton
// [*compiler.Compiler] from [orch.Module] (orch deploy forms included).
func Module() dix.Module {
	return dix.NewModule(
		"orch-lsp",
		dix.Hooks(
			dix.OnStart2(func(ctx context.Context, logger *slog.Logger, c *compiler.Compiler) error {
				logger.Info("lifecycle", "phase", "starting", "component", "orch-lsp", "transport", "stdio")
				go func() {
					if err := plsp.ServeStdio(ctx, plsp.ServerOptions{Compiler: c}); err != nil {
						logger.Error("lifecycle", "phase", "lsp_stopped", "component", "orch-lsp", "error", err)
					}
				}()
				logger.Info("lifecycle", "phase", "started", "component", "orch-lsp")
				return nil
			}),
		),
	)
}
