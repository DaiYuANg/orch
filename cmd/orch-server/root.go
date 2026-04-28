package main

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/arcgolabs/dix"
	"github.com/spf13/cobra"

	"github.com/daiyuang/orch/internal/api"
	"github.com/daiyuang/orch/internal/buildmeta"
	"github.com/daiyuang/orch/internal/config"
	"github.com/daiyuang/orch/internal/dnssvc"
	"github.com/daiyuang/orch/internal/httpserver"
	"github.com/daiyuang/orch/internal/ingress"
	"github.com/daiyuang/orch/internal/logging"
	"github.com/daiyuang/orch/internal/metrics"
	"github.com/daiyuang/orch/internal/nodeid"
	"github.com/daiyuang/orch/internal/observability"
	"github.com/daiyuang/orch/internal/raftsvc"
	"github.com/daiyuang/orch/internal/runtime"
	"github.com/daiyuang/orch/internal/scheduler"
	securityauth "github.com/daiyuang/orch/internal/security/auth"
	"github.com/daiyuang/orch/internal/services"
	"github.com/daiyuang/orch/internal/startupinfo"
)

// serverRunner wires Cobra lifecycle: PreRun builds the dix graph; Run starts it and blocks until shutdown.
type serverRunner struct {
	app *dix.App
}

func newRootCmd() *cobra.Command {
	var srv serverRunner

	cmd := &cobra.Command{
		Use:          "orch-server",
		Short:        "Orch control plane server",
		Long:         "Runs the orch HTTP API, DNS, ingress, Raft, scheduler, and related services.",
		Version:      buildmeta.Version(),
		PreRunE:      srv.preRun,
		RunE:         srv.run,
		SilenceUsage: true,
	}

	cmd.Flags().String("config", "", "Path to YAML, JSON, or TOML config file (merged before env; CLI flags override)")
	config.BindOrchFlags(cmd.Flags(), config.Default())

	return cmd
}

func (srv *serverRunner) preRun(cmd *cobra.Command, _ []string) error {
	cfg, err := config.LoadFromCobra(cmd)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	srv.app = dix.New(
		"orch-server",
		dix.WithVersion(buildmeta.Version()),
		dix.WithLoggerFrom1(func(logger *slog.Logger) *slog.Logger { return logger }),
		dix.WithModules(
			config.Static(cfg),
			logging.Module(),
			nodeid.Module(),
			observability.Module(),
			metrics.Module(),
			securityauth.Module(),
			dnssvc.Module(),
			runtime.Module(),
			raftsvc.Module(),
			services.Module(),
			ingress.Module(),
			scheduler.Module(),
			httpserver.Module(),
			api.Module(),
			startupinfo.Module(),
		),
	)
	return nil
}

func (srv *serverRunner) run(cmd *cobra.Command, _ []string) error {
	ctx := cmd.Context()
	rt, err := srv.app.Start(ctx)
	if err != nil {
		return fmt.Errorf("start orch-server: %w", err)
	}
	rt.Logger().Info("lifecycle", "phase", "ready", "app", "orch-server")

	<-ctx.Done()
	rt.Logger().Info("lifecycle", "phase", "shutdown_requested", "app", "orch-server")

	shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelShutdown()
	if err := rt.Stop(shutdownCtx); err != nil {
		rt.Logger().Warn("graceful stop error", "error", err)
	}
	return nil
}
