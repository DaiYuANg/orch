package task

import (
	"context"
	"log/slog"

	deployv1 "github.com/daiyuang/orch/internal/deploy/v1alpha1"
	"github.com/daiyuang/orch/internal/metrics"
	"github.com/daiyuang/orch/internal/oopsx"
	"github.com/daiyuang/orch/internal/runtime"
	"github.com/daiyuang/orch/internal/services/registry"
)

type Service struct {
	logger   *slog.Logger
	metrics  *metrics.Service
	runtime  *runtime.Manager
	registry *registry.Service
}

func NewService(logger *slog.Logger, metricService *metrics.Service, runtimeManager *runtime.Manager, registryService *registry.Service) *Service {
	return &Service{
		logger:   logger,
		metrics:  metricService,
		runtime:  runtimeManager,
		registry: registryService,
	}
}

func (s *Service) DeployApp(ctx context.Context, app *deployv1.App) error {
	if err := app.Validate(); err != nil {
		s.metrics.IncDeployApp(ctx, "invalid")
		return err
	}

	for _, w := range app.Workloads {
		if err := s.runtime.Deploy(ctx, app.Metadata, w); err != nil {
			s.metrics.IncDeployWorkload(ctx, string(w.Runtime), "failed")
			s.metrics.IncDeployApp(ctx, "failed")
			return oopsx.B("task").Wrapf(err, "deploy workload %s", w.Name)
		}
		s.metrics.IncDeployWorkload(ctx, string(w.Runtime), "success")
		s.registry.Upsert(registry.WorkloadRecord{
			Name:    w.Name,
			Runtime: string(w.Runtime),
			Image:   w.Run.Image,
			Status:  "running",
		})
	}
	s.metrics.IncDeployApp(ctx, "success")
	s.logger.Info("application deployed", "app", app.Metadata.Name, "workloads", len(app.Workloads))
	return nil
}
