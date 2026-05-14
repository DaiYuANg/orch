package scheduler

import (
	"context"
	"log/slog"

	"github.com/arcgolabs/dix"

	"github.com/lyonbrown4d/orch/internal/lifecycleplan"
	"github.com/lyonbrown4d/orch/internal/nodecapacity"
	"github.com/lyonbrown4d/orch/internal/nodeid"
)

// startDeps bundles scheduler startup dependencies for a single OnStart hook.
type startDeps struct {
	Logger    *slog.Logger
	Service   *Service
	Catalog   *nodecapacity.Catalog
	LocalNode nodeid.Local
}

func newStartDeps(logger *slog.Logger, s *Service, cat *nodecapacity.Catalog, local nodeid.Local) startDeps {
	return startDeps{Logger: logger, Service: s, Catalog: cat, LocalNode: local}
}

func Module() dix.Module {
	return dix.NewModule(
		"scheduler",
		dix.Providers(
			dix.ProviderErr3(New, dix.Eager()),
			dix.Provider4(newStartDeps),
		),
		dix.Hooks(
			dix.OnStart(func(ctx context.Context, d startDeps) error {
				d.Logger.Info("lifecycle", "phase", "starting", "component", "scheduler")
				if err := RegisterHeartbeatJob(d.Service); err != nil {
					d.Logger.Error("lifecycle", "phase", "start_failed", "component", "scheduler", "error", err)
					return err
				}
				if err := RegisterResourceSnapshotJob(ctx, d.Service, d.Catalog, d.LocalNode); err != nil {
					d.Logger.Error("lifecycle", "phase", "start_failed", "component", "scheduler", "error", err)
					return err
				}
				if err := d.Service.Start(ctx); err != nil {
					d.Logger.Error("lifecycle", "phase", "start_failed", "component", "scheduler", "error", err)
					return err
				}
				d.Logger.Info("lifecycle", "phase", "started", "component", "scheduler")
				return nil
			}, dix.LifecycleName(lifecycleplan.HookScheduler), dix.LifecyclePriority(lifecycleplan.PriorityWorkload), dix.LifecycleParallel(), dix.LifecycleTimeout(lifecycleplan.TimeoutStart)),
			dix.OnStop2(func(ctx context.Context, logger *slog.Logger, s *Service) error {
				logger.Info("lifecycle", "phase", "stopping", "component", "scheduler")
				if err := s.Stop(ctx); err != nil {
					logger.Warn("lifecycle", "phase", "stop_failed", "component", "scheduler", "error", err)
					return err
				}
				logger.Info("lifecycle", "phase", "stopped", "component", "scheduler")
				return nil
			}, dix.LifecycleName(lifecycleplan.HookScheduler), dix.LifecyclePriority(lifecycleplan.PriorityWorkload), dix.LifecycleParallel(), dix.LifecycleTimeout(lifecycleplan.TimeoutStop)),
		),
	)
}
