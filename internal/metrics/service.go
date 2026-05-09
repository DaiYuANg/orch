package metrics

import (
	"context"
	"time"

	"github.com/arcgolabs/dix"
	obs "github.com/arcgolabs/observabilityx"

	"github.com/daiyuang/orch/internal/observability"
)

type Service struct {
	deployAppCounter      obs.Counter
	deployWorkloadCounter obs.Counter
	dixEventCounter       obs.Counter
	dixDurationHistogram  obs.Histogram
}

var (
	_ dix.Observer              = (*Service)(nil)
	_ dix.ProviderObserver      = (*Service)(nil)
	_ dix.ResolveObserver       = (*Service)(nil)
	_ dix.LifecycleHookObserver = (*Service)(nil)
)

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
		dixEventCounter: backend.Counter(obs.NewCounterSpec(
			"orch.dix.event_total",
			obs.WithDescription("Total number of dix framework diagnostic events."),
			obs.WithLabelKeys("event", "operation", "target", "status"),
		)),
		dixDurationHistogram: backend.Histogram(obs.NewHistogramSpec(
			"orch.dix.event_duration_ms",
			obs.WithDescription("Duration of dix framework build, lifecycle, provider, and resolve events."),
			obs.WithUnit("ms"),
			obs.WithLabelKeys("event", "operation", "target", "status"),
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

func (s *Service) OnBuild(ctx context.Context, event dix.BuildEvent) {
	s.recordDixDuration(ctx, "build", "", event.Meta.Name, event.Duration, event.Err)
}

func (s *Service) OnStart(ctx context.Context, event dix.StartEvent) {
	s.recordDixDuration(ctx, "start", "", event.Meta.Name, event.Duration, event.Err)
}

func (s *Service) OnStop(ctx context.Context, event dix.StopEvent) {
	s.recordDixDuration(ctx, "stop", "", event.Meta.Name, event.Duration, event.Err)
}

func (s *Service) OnHealthCheck(ctx context.Context, event dix.HealthCheckEvent) {
	s.recordDixDuration(ctx, "health_check", string(event.Kind), event.Name, event.Duration, event.Err)
}

func (s *Service) OnStateTransition(ctx context.Context, event dix.StateTransitionEvent) {
	s.recordDixEvent(ctx, "state_transition", event.From.String()+"->"+event.To.String(), event.Meta.Name, nil)
}

func (s *Service) OnProvider(ctx context.Context, event dix.ProviderEvent) {
	s.recordDixDuration(ctx, "provider", event.Operation, event.Service, event.Duration, event.Err)
}

func (s *Service) OnResolve(ctx context.Context, event dix.ResolveEvent) {
	s.recordDixDuration(ctx, "resolve", event.Operation, event.Service, event.Duration, event.Err)
}

func (s *Service) OnLifecycleHook(ctx context.Context, event dix.LifecycleHookEvent) {
	target := event.Name
	if target == "" {
		target = event.Label
	}
	s.recordDixDuration(ctx, "lifecycle_hook", string(event.Kind), target, event.Duration, event.Err)
}

func (s *Service) recordDixDuration(ctx context.Context, event, operation, target string, duration time.Duration, err error) {
	s.recordDixEvent(ctx, event, operation, target, err)
	s.dixDurationHistogram.Record(ctx, float64(duration)/float64(time.Millisecond),
		obs.String("event", event),
		obs.String("operation", operation),
		obs.String("target", target),
		obs.String("status", dixStatus(err)),
	)
}

func (s *Service) recordDixEvent(ctx context.Context, event, operation, target string, err error) {
	s.dixEventCounter.Add(ctx, 1,
		obs.String("event", event),
		obs.String("operation", operation),
		obs.String("target", target),
		obs.String("status", dixStatus(err)),
	)
}

func dixStatus(err error) string {
	if err != nil {
		return "error"
	}
	return "ok"
}
