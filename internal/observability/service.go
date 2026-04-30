package observability

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"

	obs "github.com/arcgolabs/observabilityx"
	obsprom "github.com/arcgolabs/observabilityx/prometheus"
	prom "github.com/prometheus/client_golang/prometheus"

	"github.com/daiyuang/orch/internal/config"
)

type Service struct {
	backend obs.Observability
	prom    *obsprom.Adapter
	reg     *prom.Registry // set when Prometheus enabled; shared with Fiber prometheus middleware

	shutdownOTLP func(context.Context) error
}

func New(cfg config.Config, reg *prom.Registry, logger *slog.Logger) (*Service, error) {
	promOn := cfg.Observability.Prometheus.Enabled
	otlpOn := cfg.Observability.OTLP.Enabled

	if !promOn && !otlpOn {
		return &Service{backend: obs.Nop()}, nil
	}

	var backends []obs.Observability
	svc := &Service{}

	if otlpOn {
		otelBackend, shutdown, err := newOTLP(context.Background(), cfg, logger)
		if err != nil {
			return nil, err
		}
		svc.shutdownOTLP = shutdown
		backends = append(backends, otelBackend)
	}

	if promOn {
		if reg == nil {
			if svc.shutdownOTLP != nil {
				_ = svc.shutdownOTLP(context.Background())
			}
			return nil, fmt.Errorf("observability: prometheus enabled but registry is nil")
		}
		p := obsprom.New(
			obsprom.WithNamespace("orch"),
			obsprom.WithRegisterer(reg),
			obsprom.WithGatherer(reg),
		)
		backends = append(backends, p)
		svc.prom = p
		svc.reg = reg
	}

	switch len(backends) {
	case 0:
		svc.backend = obs.Nop()
	case 1:
		svc.backend = backends[0]
	default:
		svc.backend = obs.Multi(backends...)
	}

	return svc, nil
}

func (s *Service) Backend() obs.Observability {
	return s.backend
}

// Shutdown flushes OTLP exporters when OTLP was enabled. Safe to call multiple times.
func (s *Service) Shutdown(ctx context.Context) error {
	if s.shutdownOTLP == nil {
		return nil
	}
	err := s.shutdownOTLP(ctx)
	s.shutdownOTLP = nil
	return err
}

// PrometheusRegistry returns the shared registry used by observabilityx metrics when enabled.
func (s *Service) PrometheusRegistry() *prom.Registry {
	return s.reg
}

func (s *Service) MetricsHandler() http.Handler {
	if s.prom == nil {
		return nil
	}
	return s.prom.Handler()
}
