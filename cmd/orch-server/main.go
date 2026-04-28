package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/arcgolabs/logx"

	"github.com/daiyuang/orch/internal/config"
	"github.com/daiyuang/orch/internal/logging"
)

func main() {
	// Bootstrap slog for errors before/during Cobra (same defaults as config; app uses merged config from dix).
	bootstrap, err := logging.New(config.Default().Log)
	if err != nil {
		slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo})))
	} else {
		slog.SetDefault(bootstrap)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	runErr := newRootCmd().ExecuteContext(ctx)
	if runErr != nil {
		slog.Default().Error("start application failed", "error", runErr)
	}
	if bootstrap != nil {
		_ = logx.Close(bootstrap)
	}
	if runErr != nil {
		os.Exit(1)
	}
}
