package httpserver

import (
	"strings"

	fiberprometheus "github.com/ansrivas/fiberprometheus/v2"
	"github.com/gofiber/fiber/v2"

	"github.com/daiyuang/orch/internal/config"
	"github.com/daiyuang/orch/internal/observability"
)

// attachFiberPrometheus registers github.com/ansrivas/fiberprometheus/v2 HTTP middleware (requests_total,
// latency histograms, in-flight) and the scrape route on the shared Prometheus registry from observability,
// so orch_* application metrics and http_fiber_* HTTP metrics export from one endpoint.
func attachFiberPrometheus(app *fiber.App, cfg config.Config, obs *observability.Service) {
	reg := obs.PrometheusRegistry()
	if reg == nil {
		return
	}

	path := strings.TrimSpace(cfg.Observability.PrometheusPath)
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

	fp := fiberprometheus.NewWithRegistry(reg, serviceName, "http", "fiber", nil)
	fp.SetSkipPaths([]string{path})
	fp.RegisterAt(app, path)
	app.Use(fp.Middleware)
}
