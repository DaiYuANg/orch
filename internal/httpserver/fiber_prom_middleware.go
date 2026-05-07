// Copyright (c) 2021-present Ankur Srivastava and Contributors
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.
//
// Derived from github.com/ansrivas/fiberprometheus/v2 v2.17.0: same HTTP metrics and
// middleware behavior, with an optional Prometheus native histogram on request duration.

package httpserver

import (
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/arcgolabs/collectionx/list"
	"github.com/arcgolabs/collectionx/set"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/adaptor"
	"github.com/gofiber/fiber/v2/utils"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/otel/trace"
)

const defaultNativeHistogramBucketFactor = 1.1

type fiberPromMetrics struct {
	gatherer          prometheus.Gatherer
	requestsTotal     *prometheus.CounterVec
	requestDuration   *prometheus.HistogramVec
	requestInFlight   *prometheus.GaugeVec
	defaultURL        string
	skipPaths         *set.Set[string]
	ignoreStatusCodes map[int]bool
	registeredRoutes  *set.Set[string]
	routesOnce        sync.Once
}

func newFiberPromMetrics(registry prometheus.Registerer, serviceName, namespace, subsystem string, labels map[string]string, nativeHistogram bool) *fiberPromMetrics {
	if registry == nil {
		registry = prometheus.NewRegistry()
	}

	constLabels := make(prometheus.Labels)
	if serviceName != "" {
		constLabels["service"] = serviceName
	}
	for label, value := range labels {
		constLabels[label] = value
	}

	counter := promauto.With(registry).NewCounterVec(
		prometheus.CounterOpts{
			Name:        prometheus.BuildFQName(namespace, subsystem, "requests_total"),
			Help:        "Count all http requests by status code, method and path.",
			ConstLabels: constLabels,
		},
		[]string{"status_code", "method", "path"},
	)

	histogramOpts := prometheus.HistogramOpts{
		Name:        prometheus.BuildFQName(namespace, subsystem, "request_duration_seconds"),
		Help:        "Duration of all HTTP requests by status code, method and path.",
		ConstLabels: constLabels,
		Buckets: []float64{
			0.000000001, // 1ns
			0.000000002,
			0.000000005,
			0.00000001, // 10ns
			0.00000002,
			0.00000005,
			0.0000001, // 100ns
			0.0000002,
			0.0000005,
			0.000001, // 1µs
			0.000002,
			0.000005,
			0.00001, // 10µs
			0.00002,
			0.00005,
			0.0001, // 100µs
			0.0002,
			0.0005,
			0.001, // 1ms
			0.002,
			0.005,
			0.01, // 10ms
			0.02,
			0.05,
			0.1, // 100 ms
			0.2,
			0.5,
			1.0, // 1s
			2.0,
			5.0,
			10.0, // 10s
			15.0,
			20.0,
			30.0,
			60.0, // 1m
		},
	}
	if nativeHistogram {
		histogramOpts.NativeHistogramBucketFactor = defaultNativeHistogramBucketFactor
	}

	histogram := promauto.With(registry).NewHistogramVec(histogramOpts,
		[]string{"status_code", "method", "path"},
	)

	gauge := promauto.With(registry).NewGaugeVec(prometheus.GaugeOpts{
		Name:        prometheus.BuildFQName(namespace, subsystem, "requests_in_progress_total"),
		Help:        "All the requests in progress",
		ConstLabels: constLabels,
	}, []string{"method"})

	gatherer, ok := registry.(prometheus.Gatherer)
	if !ok {
		gatherer = prometheus.DefaultGatherer
	}

	return &fiberPromMetrics{
		gatherer:        gatherer,
		requestsTotal:   counter,
		requestDuration: histogram,
		requestInFlight: gauge,
		defaultURL:      "/metrics",
	}
}

func (ps *fiberPromMetrics) registerAt(app fiber.Router, url string, handlers ...fiber.Handler) {
	ps.defaultURL = url

	h := append(handlers, adaptor.HTTPHandler(promhttp.HandlerFor(ps.gatherer, promhttp.HandlerOpts{
		EnableOpenMetrics: true,
	})))
	app.Get(ps.defaultURL, h...)
}

func (ps *fiberPromMetrics) setSkipPaths(paths *list.List[string]) {
	if ps.skipPaths == nil {
		ps.skipPaths = set.NewSet[string]()
	}
	paths.Range(func(_ int, path string) bool {
		ps.skipPaths.Add(path)
		return true
	})
}

func (ps *fiberPromMetrics) Middleware(ctx *fiber.Ctx) error {
	method := utils.CopyString(ctx.Method())

	ps.requestInFlight.WithLabelValues(method).Inc()
	defer func() {
		ps.requestInFlight.WithLabelValues(method).Dec()
	}()

	start := time.Now()

	err := ctx.Next()

	routePath := utils.CopyString(ctx.Route().Path)

	if routePath == "/" {
		routePath = utils.CopyString(ctx.Path())
	}

	if routePath != "" && routePath != "/" {
		routePath = normalizePromRoutePath(routePath)
	}

	ps.routesOnce.Do(func() {
		ps.registeredRoutes = set.NewSet[string]()
		for _, r := range ctx.App().GetRoutes(true) {
			p := r.Path
			if p != "" && p != "/" {
				p = normalizePromRoutePath(p)
			}
			ps.registeredRoutes.Add(r.Method + " " + p)
		}
	})

	if !ps.registeredRoutes.Contains(method + " " + routePath) {
		return err
	}

	if ps.skipPaths.Contains(routePath) {
		return nil
	}

	status := fiber.StatusInternalServerError
	if err != nil {
		if e, ok := err.(*fiber.Error); ok {
			status = e.Code
		}
	} else {
		status = ctx.Response().StatusCode()
	}

	statusCode := strconv.Itoa(status)

	if ps.ignoreStatusCodes[status] {
		return err
	}

	ps.requestsTotal.WithLabelValues(statusCode, method, routePath).Inc()

	elapsed := float64(time.Since(start).Nanoseconds()) / 1e9

	traceID := trace.SpanContextFromContext(ctx.UserContext()).TraceID()
	histogram := ps.requestDuration.WithLabelValues(statusCode, method, routePath)

	if traceID.IsValid() {
		if histogramExemplar, ok := histogram.(prometheus.ExemplarObserver); ok {
			histogramExemplar.ObserveWithExemplar(elapsed, prometheus.Labels{"traceID": traceID.String()})
		}

		return err
	}

	histogram.Observe(elapsed)

	return err
}

func normalizePromRoutePath(routePath string) string {
	normalized := strings.TrimRight(routePath, "/")
	if normalized == "" {
		return "/"
	}
	return normalized
}
