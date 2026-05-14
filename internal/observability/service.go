package observability

import (
	"context"
	"errors"
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
	if observabilityDisabled(cfg) {
		return &Service{backend: obs.Nop()}, nil
	}
	svc := &Service{}
	backends, err := svc.configureBackends(cfg, reg, logger)
	if err != nil {
		return nil, err
	}
	svc.backend = combineBackends(backends)
	return svc, nil
}

func observabilityDisabled(cfg config.Config) bool {
	return !cfg.Observability.Prometheus.Enabled && !cfg.Observability.OTLP.Enabled
}

func (s *Service) configureBackends(cfg config.Config, reg *prom.Registry, logger *slog.Logger) ([]obs.Observability, error) {
	var backends []obs.Observability
	if cfg.Observability.OTLP.Enabled {
		backend, shutdown, err := newOTLP(context.Background(), cfg, logger)
		if err != nil {
			return nil, err
		}
		s.shutdownOTLP = shutdown
		backends = append(backends, backend)
	}
	if cfg.Observability.Prometheus.Enabled {
		backend, err := s.configurePrometheus(reg)
		if err != nil {
			return nil, err
		}
		backends = append(backends, backend)
	}
	return backends, nil
}

func (s *Service) configurePrometheus(reg *prom.Registry) (obs.Observability, error) {
	if reg == nil {
		err := errors.New("observability: prometheus enabled but registry is nil")
		if s.shutdownOTLP != nil {
			err = errors.Join(err, s.shutdownOTLP(context.Background()))
		}
		return nil, err
	}
	prometheus := obsprom.New(
		obsprom.WithNamespace("orch"),
		obsprom.WithRegisterer(reg),
		obsprom.WithGatherer(reg),
	)
	s.prom = prometheus
	s.reg = reg
	return prometheus, nil
}

func combineBackends(backends []obs.Observability) obs.Observability {
	switch len(backends) {
	case 0:
		return obs.Nop()
	case 1:
		return backends[0]
	default:
		return obs.Multi(backends...)
	}
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
