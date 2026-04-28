package observability

import (
	prom "github.com/prometheus/client_golang/prometheus"

	"github.com/daiyuang/orch/internal/config"
)

// NewPrometheusRegistry allocates an isolated Prometheus registry when metrics export is enabled.
// Returns nil when Prometheus is disabled so downstream wiring can stay explicit.
func NewPrometheusRegistry(cfg config.Config) *prom.Registry {
	if !cfg.Observability.Prometheus.Enabled {
		return nil
	}
	return prom.NewRegistry()
}
