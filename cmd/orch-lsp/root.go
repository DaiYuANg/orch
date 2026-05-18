package main

import (
	"context"
	"fmt"
	"time"

	"github.com/arcgolabs/dix"
	"github.com/spf13/cobra"

	"github.com/lyonbrown4d/orch/internal/buildmeta"
	"github.com/lyonbrown4d/orch/internal/config"
	"github.com/lyonbrown4d/orch/internal/deploy/orch"
	"github.com/lyonbrown4d/orch/internal/logging"
	"github.com/lyonbrown4d/orch/internal/orchlsp"
)

type lspRunner struct {
	app *dix.App
}

func newRootCmd() *cobra.Command {
	var run lspRunner

	cmd := &cobra.Command{
		Use:          "orch-lsp",
		Short:        "Language server for .orch (stdio LSP)",
		Long:         "Runs the plano LSP stack against an orch-configured compiler (deploy forms registered). Intended for editor integration over standard I/O.",
		Version:      buildmeta.Version(),
		PreRunE:      run.preRun,
		RunE:         run.run,
		SilenceUsage: true,
	}

	cmd.Flags().String("config", "", "Path to YAML, JSON, TOML, or HCL config file (log/sink only; optional)")
	config.BindOrchFlags(cmd.Flags(), config.Default())

	return cmd
}

func (r *lspRunner) preRun(cmd *cobra.Command, _ []string) error {
	cfg, err := config.LoadFromCobra(cmd)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	r.app = dix.New(
		"orch-lsp",
		dix.Modules(
			buildmeta.Module(),
			config.Static(cfg),
			logging.Module(),
			orch.Module(),
			orchlsp.Module(),
		),
	)
	return nil
}

func (r *lspRunner) run(cmd *cobra.Command, _ []string) error {
	ctx := cmd.Context()
	rt, err := r.app.Start(ctx)
	if err != nil {
		return fmt.Errorf("start orch-lsp: %w", err)
	}
	rt.Logger().Info("lifecycle", "phase", "ready", "app", "orch-lsp")

	<-ctx.Done()
	rt.Logger().Info("lifecycle", "phase", "shutdown_requested", "app", "orch-lsp")

	shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelShutdown()
	report, err := rt.StopWithReport(shutdownCtx)
	if err != nil {
		rt.Logger().Warn("graceful stop error", "error", err)
	} else if report != nil && report.HasErrors() {
		rt.Logger().Warn("graceful stop error", "error", report.Err())
	}
	return nil
}
