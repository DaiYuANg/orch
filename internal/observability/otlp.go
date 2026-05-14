package observability

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	obs "github.com/arcgolabs/observabilityx"
	obsotel "github.com/arcgolabs/observabilityx/otel"
	"github.com/samber/lo"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"

	"github.com/lyonbrown4d/orch/internal/config"
)

const (
	defaultOTLPExportInterval = 10 * time.Second
	orchInstrumentationName   = "github.com/lyonbrown4d/orch"
)

func newOTLP(ctx context.Context, cfg config.Config, logger *slog.Logger) (obs.Observability, func(context.Context) error, error) {
	o := cfg.Observability.OTLP
	proto := lo.CoalesceOrEmpty(strings.ToLower(strings.TrimSpace(o.Protocol)), "grpc")
	serviceName := lo.CoalesceOrEmpty(strings.TrimSpace(o.ServiceName), strings.TrimSpace(cfg.App.Name), "orch")

	res, err := resource.New(ctx, resource.WithAttributes(semconv.ServiceName(serviceName)))
	if err != nil {
		return nil, nil, fmt.Errorf("otlp resource: %w", err)
	}
	exporters, err := newOTLPExporters(ctx, proto, cfg)
	if err != nil {
		return nil, nil, err
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporters.trace),
		sdktrace.WithResource(res),
	)
	otel.SetTracerProvider(tp)

	reader := sdkmetric.NewPeriodicReader(exporters.metric, sdkmetric.WithInterval(defaultOTLPExportInterval))
	mp := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(reader),
		sdkmetric.WithResource(res),
	)
	otel.SetMeterProvider(mp)

	shutdown := func(shutdownCtx context.Context) error {
		var errs []error
		if err := mp.Shutdown(shutdownCtx); err != nil {
			errs = append(errs, fmt.Errorf("meter provider shutdown: %w", err))
		}
		if err := tp.Shutdown(shutdownCtx); err != nil {
			errs = append(errs, fmt.Errorf("tracer provider shutdown: %w", err))
		}
		return errors.Join(errs...)
	}

	backend := obsotel.New(
		obsotel.WithLogger(logger),
		obsotel.WithTracer(tp.Tracer(orchInstrumentationName)),
		obsotel.WithMeter(mp.Meter(orchInstrumentationName)),
	)
	return backend, shutdown, nil
}

type otlpExporters struct {
	trace  sdktrace.SpanExporter
	metric sdkmetric.Exporter
}

func newOTLPExporters(ctx context.Context, proto string, cfg config.Config) (otlpExporters, error) {
	switch proto {
	case "grpc":
		return newOTLPGRPCExporters(ctx, cfg)
	case "http":
		return newOTLPHTTPExporters(ctx, cfg)
	default:
		return otlpExporters{}, fmt.Errorf("otlp: unknown protocol %q (use grpc or http)", cfg.Observability.OTLP.Protocol)
	}
}

func newOTLPGRPCExporters(ctx context.Context, cfg config.Config) (otlpExporters, error) {
	traceOptions, metricOptions := otlpGRPCOptions(cfg)
	traceExporter, err := otlptracegrpc.New(ctx, traceOptions...)
	if err != nil {
		return otlpExporters{}, fmt.Errorf("otlp grpc trace exporter: %w", err)
	}
	metricExporter, err := otlpmetricgrpc.New(ctx, metricOptions...)
	if err != nil {
		return otlpExporters{}, errors.Join(
			fmt.Errorf("otlp grpc metric exporter: %w", err),
			shutdownTraceExporter(ctx, traceExporter),
		)
	}
	return otlpExporters{trace: traceExporter, metric: metricExporter}, nil
}

func otlpGRPCOptions(cfg config.Config) ([]otlptracegrpc.Option, []otlpmetricgrpc.Option) {
	otlp := cfg.Observability.OTLP
	hostport := otlpGRPCAddr(otlp.Endpoint)
	traceOptions := []otlptracegrpc.Option{otlptracegrpc.WithEndpoint(hostport)}
	metricOptions := []otlpmetricgrpc.Option{otlpmetricgrpc.WithEndpoint(hostport)}
	if otlp.Insecure {
		traceOptions = append(traceOptions, otlptracegrpc.WithInsecure())
		metricOptions = append(metricOptions, otlpmetricgrpc.WithInsecure())
	}
	return traceOptions, metricOptions
}

func newOTLPHTTPExporters(ctx context.Context, cfg config.Config) (otlpExporters, error) {
	otlp := cfg.Observability.OTLP
	traceExporter, err := otlptracehttp.New(ctx, otlpHTTPTraceOptions(otlp.Endpoint, otlp.Insecure)...)
	if err != nil {
		return otlpExporters{}, fmt.Errorf("otlp http trace exporter: %w", err)
	}
	metricExporter, err := otlpmetrichttp.New(ctx, otlpHTTPMetricOptions(otlp.Endpoint, otlp.Insecure)...)
	if err != nil {
		return otlpExporters{}, errors.Join(
			fmt.Errorf("otlp http metric exporter: %w", err),
			shutdownTraceExporter(ctx, traceExporter),
		)
	}
	return otlpExporters{trace: traceExporter, metric: metricExporter}, nil
}

func shutdownTraceExporter(ctx context.Context, exporter sdktrace.SpanExporter) error {
	if exporter == nil {
		return nil
	}
	if err := exporter.Shutdown(ctx); err != nil {
		return fmt.Errorf("otlp trace exporter shutdown: %w", err)
	}
	return nil
}

func otlpGRPCAddr(endpoint string) string {
	return stripScheme(lo.CoalesceOrEmpty(strings.TrimSpace(endpoint), "localhost:4317"))
}

func stripScheme(hostport string) string {
	s := strings.TrimSpace(hostport)
	for _, prefix := range []string{"http://", "https://", "grpc://"} {
		if strings.HasPrefix(strings.ToLower(s), prefix) {
			return strings.TrimSpace(s[len(prefix):])
		}
	}
	return s
}

func otlpHTTPTraceOptions(endpoint string, insecure bool) []otlptracehttp.Option {
	return otlpHTTPOptions(endpoint, insecure, otlptracehttp.WithEndpointURL, otlptracehttp.WithEndpoint, otlptracehttp.WithInsecure)
}

func otlpHTTPMetricOptions(endpoint string, insecure bool) []otlpmetrichttp.Option {
	return otlpHTTPOptions(endpoint, insecure, otlpmetrichttp.WithEndpointURL, otlpmetrichttp.WithEndpoint, otlpmetrichttp.WithInsecure)
}

func otlpHTTPOptions[T any](
	endpoint string,
	insecure bool,
	withURL func(string) T,
	withHostPort func(string) T,
	withInsecure func() T,
) []T {
	e := strings.TrimSpace(endpoint)
	if e == "" {
		e = "http://localhost:4318"
	} else if !strings.Contains(e, "://") {
		if insecure {
			e = "http://" + e
		} else {
			e = "https://" + e
		}
	}
	if strings.Contains(e, "://") {
		out := []T{withURL(e)}
		if insecure && strings.HasPrefix(strings.ToLower(e), "http://") {
			out = append(out, withInsecure())
		}
		return out
	}
	out := []T{withHostPort(e)}
	if insecure {
		out = append(out, withInsecure())
	}
	return out
}
