package main

import (
	"context"
	"fmt"
	"time"

	"github.com/arcgolabs/dix"
	"github.com/spf13/cobra"

	"github.com/lyonbrown4d/orch/internal/api"
	"github.com/lyonbrown4d/orch/internal/buildmeta"
	"github.com/lyonbrown4d/orch/internal/config"
	"github.com/lyonbrown4d/orch/internal/deploy/loader"
	"github.com/lyonbrown4d/orch/internal/deploy/orch"
	"github.com/lyonbrown4d/orch/internal/dixdiag"
	"github.com/lyonbrown4d/orch/internal/dnssvc"
	"github.com/lyonbrown4d/orch/internal/httpserver"
	"github.com/lyonbrown4d/orch/internal/ingress"
	"github.com/lyonbrown4d/orch/internal/lifecycleplan"
	"github.com/lyonbrown4d/orch/internal/logging"
	"github.com/lyonbrown4d/orch/internal/metrics"
	"github.com/lyonbrown4d/orch/internal/nodeid"
	"github.com/lyonbrown4d/orch/internal/observability"
	"github.com/lyonbrown4d/orch/internal/orchvpn"
	"github.com/lyonbrown4d/orch/internal/raftsvc"
	"github.com/lyonbrown4d/orch/internal/runtime"
	"github.com/lyonbrown4d/orch/internal/scheduler"
	securityauth "github.com/lyonbrown4d/orch/internal/security/auth"
	"github.com/lyonbrown4d/orch/internal/services"
	"github.com/lyonbrown4d/orch/internal/startupinfo"
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
	cmd.AddCommand(newHostDNSCmd())

	return cmd
}

func (srv *serverRunner) preRun(cmd *cobra.Command, _ []string) error {
	cfg, err := config.LoadFromCobra(cmd)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	srv.app = dix.New(
		"orch-server",
		dix.LifecycleConcurrency(lifecycleplan.Concurrency),
		dix.RecentEvents(lifecycleplan.RecentEventCapacity),
		dix.Modules(
			buildmeta.Module(),
			config.Static(cfg),
			logging.Module(),
			orch.Module(),
			loader.Module(),
			nodeid.Module(),
			observability.Module(),
			metrics.Module(),
			dixdiag.Module(),
			securityauth.Module(),
			dnssvc.Module(),
			orchvpn.GatewayModule(),
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
	if diag, resolveErr := dix.ResolveAsContext[*dixdiag.Service](ctx, rt.Container()); resolveErr != nil {
		rt.Logger().Warn("dix diagnostics service unavailable", "error", resolveErr)
	} else {
		diag.Attach(rt)
	}
	logDixRuntimeDiagnostics(rt)
	rt.Logger().Info("orch-server ready (control plane running; Ctrl+C to stop)")

	<-ctx.Done()
	rt.Logger().Info("orch-server shutdown requested")

	shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelShutdown()
	if err := rt.Stop(shutdownCtx); err != nil {
		rt.Logger().Warn("orch-server graceful stop error", "error", err)
	}
	return nil
}

func logDixRuntimeDiagnostics(rt *dix.Runtime) {
	if rt == nil || rt.Logger() == nil {
		return
	}

	summary := rt.LifecycleSummary()
	events := rt.RecentEvents()
	var buildDuration time.Duration
	var startDuration time.Duration
	events.Range(func(_ int, record dix.EventRecord) bool {
		switch event := record.Event.(type) {
		case dix.BuildEvent:
			buildDuration = event.Duration
		case dix.StartEvent:
			startDuration = event.Duration
		}
		return true
	})

	rt.Logger().Info("dix runtime diagnostics",
		"lifecycle_start_hooks", summary.StartHooks,
		"lifecycle_stop_hooks", summary.StopHooks,
		"lifecycle_concurrency", summary.Concurrency,
		"recent_event_capacity", lifecycleplan.RecentEventCapacity,
		"recent_events", events.Len(),
		"build_duration", buildDuration,
		"start_duration", startDuration,
	)
}
