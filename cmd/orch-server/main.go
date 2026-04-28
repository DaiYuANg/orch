package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/arcgolabs/dix"
	"github.com/arcgolabs/logx"

	"github.com/daiyuang/orch/internal/api"
	"github.com/daiyuang/orch/internal/config"
	"github.com/daiyuang/orch/internal/dnssvc"
	"github.com/daiyuang/orch/internal/httpserver"
	"github.com/daiyuang/orch/internal/ingress"
	"github.com/daiyuang/orch/internal/logging"
	"github.com/daiyuang/orch/internal/metrics"
	"github.com/daiyuang/orch/internal/observability"
	"github.com/daiyuang/orch/internal/raftsvc"
	"github.com/daiyuang/orch/internal/runtime"
	"github.com/daiyuang/orch/internal/scheduler"
	securityauth "github.com/daiyuang/orch/internal/security/auth"
	"github.com/daiyuang/orch/internal/services"
)

func main() {
	ctx, cancelSignal := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancelSignal()

	// Framework lifecycle/build/debug logs use the same logx-backed *slog.Logger as DI (see logging.Module).
	application := dix.New(
		"orch-server",
		dix.WithVersion("v0.1.0"),
		dix.WithLoggerFrom1(func(logger *slog.Logger) *slog.Logger { return logger }),
		dix.WithModules(
			config.Module(),
			logging.Module(),
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
		),
	)
	rt, err := application.Start(ctx)
	if err != nil {
		reportExitError(err)
		os.Exit(1)
	}
	rt.Logger().Info("lifecycle", "phase", "ready", "app", "orch-server")
	config.LogReachableEndpoints(rt.Logger(), loadConfigOrDefault())

	<-ctx.Done()
	rt.Logger().Info("lifecycle", "phase", "shutdown_requested", "app", "orch-server")

	shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelShutdown()
	if err := rt.Stop(shutdownCtx); err != nil {
		reportShutdownError(err)
	}
}

func loadConfigOrDefault() config.Config {
	cfg, err := config.Load()
	if err != nil {
		return config.Default()
	}
	return cfg
}

func reportExitError(err error) {
	lg, lerr := logging.New(loadConfigOrDefault().Log)
	if lerr != nil {
		return
	}
	defer func() { _ = logx.Close(lg) }()
	lg.Error("start application failed", "error", err)
}

func reportShutdownError(err error) {
	lg, lerr := logging.New(loadConfigOrDefault().Log)
	if lerr != nil {
		return
	}
	defer func() { _ = logx.Close(lg) }()
	lg.Warn("graceful stop error", "error", err)
}
