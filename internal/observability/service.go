package observability

import (
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
}

func New(cfg config.Config, reg *prom.Registry) *Service {
	if !cfg.Observability.PrometheusEnabled {
		return &Service{backend: obs.Nop()}
	}
	if reg == nil {
		return &Service{backend: obs.Nop()}
	}

	p := obsprom.New(
		obsprom.WithNamespace("orch"),
		obsprom.WithRegisterer(reg),
		obsprom.WithGatherer(reg),
	)
	return &Service{
		backend: p,
		prom:    p,
		reg:     reg,
	}
}

func (s *Service) Backend() obs.Observability {
	return s.backend
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
