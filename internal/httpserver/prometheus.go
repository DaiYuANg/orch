package httpserver

import (
	"strings"

	"github.com/arcgolabs/collectionx/list"
	"github.com/gofiber/fiber/v2"

	"github.com/daiyuang/orch/internal/config"
	"github.com/daiyuang/orch/internal/observability"
)

// attachFiberPrometheus registers HTTP middleware (requests_total, request duration histogram,
// requests_in_progress) and the scrape route on the shared Prometheus registry from observability,
// so orch_* application metrics and http_fiber_* metrics share one endpoint.
//
// When observability.prometheus.native_histogram is true, duration uses a hybrid classic + native
// histogram ([prometheus.HistogramOpts.NativeHistogramBucketFactor]).
func attachFiberPrometheus(app *fiber.App, cfg config.Config, obs *observability.Service) {
	reg := obs.PrometheusRegistry()
	if reg == nil {
		return
	}

	path := strings.TrimSpace(cfg.Observability.Prometheus.Path)
	if path == "" {
		path = "/metrics"
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}

	serviceName := cfg.App.Name
	if serviceName == "" {
		serviceName = "orch"
	}

	fp := newFiberPromMetrics(reg, serviceName, "http", "fiber", nil, cfg.Observability.Prometheus.NativeHistogram)
	fp.setSkipPaths(list.NewList(path).Values())
	fp.registerAt(app, path)
	app.Use(fp.Middleware)
}
