package metrics

import (
	"context"

	obs "github.com/arcgolabs/observabilityx"

	"github.com/daiyuang/orch/internal/observability"
)

type Service struct {
	deployAppCounter      obs.Counter
	deployWorkloadCounter obs.Counter
}

func New(obsv *observability.Service) *Service {
	backend := obsv.Backend()
	return &Service{
		deployAppCounter: backend.Counter(obs.NewCounterSpec(
			"orch.task.deploy_app_total",
			obs.WithDescription("Total number of app deploy requests."),
			obs.WithLabelKeys("status"),
		)),
		deployWorkloadCounter: backend.Counter(obs.NewCounterSpec(
			"orch.task.deploy_workload_total",
			obs.WithDescription("Total number of workload deploy attempts."),
			obs.WithLabelKeys("runtime", "status"),
		)),
	}
}

func (s *Service) IncDeployApp(ctx context.Context, status string) {
	s.deployAppCounter.Add(ctx, 1, obs.String("status", status))
}

func (s *Service) IncDeployWorkload(ctx context.Context, runtime, status string) {
	s.deployWorkloadCounter.Add(
		ctx,
		1,
		obs.String("runtime", runtime),
		obs.String("status", status),
	)
}
