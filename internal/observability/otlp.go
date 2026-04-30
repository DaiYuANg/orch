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
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"

	"github.com/daiyuang/orch/internal/config"
)

const (
	defaultOTLPExportInterval = 10 * time.Second
	orchInstrumentationName   = "github.com/daiyuang/orch"
)

func newOTLP(ctx context.Context, cfg config.Config, logger *slog.Logger) (obs.Observability, func(context.Context) error, error) {
	o := cfg.Observability.OTLP
	proto := strings.ToLower(strings.TrimSpace(o.Protocol))
	if proto == "" {
		proto = "grpc"
	}

	serviceName := strings.TrimSpace(o.ServiceName)
	if serviceName == "" {
		serviceName = strings.TrimSpace(cfg.App.Name)
	}
	if serviceName == "" {
		serviceName = "orch"
	}

	res, err := resource.New(ctx, resource.WithAttributes(semconv.ServiceName(serviceName)))
	if err != nil {
		return nil, nil, fmt.Errorf("otlp resource: %w", err)
	}

	var traceExporter sdktrace.SpanExporter
	var metricExporter sdkmetric.Exporter

	switch proto {
	case "grpc":
		hostport := otlpGRPCAddr(o.Endpoint)
		topts := []otlptracegrpc.Option{otlptracegrpc.WithEndpoint(hostport)}
		mopts := []otlpmetricgrpc.Option{otlpmetricgrpc.WithEndpoint(hostport)}
		if o.Insecure {
			topts = append(topts, otlptracegrpc.WithInsecure())
			mopts = append(mopts, otlpmetricgrpc.WithInsecure())
		}
		traceExporter, err = otlptracegrpc.New(ctx, topts...)
		if err != nil {
			return nil, nil, fmt.Errorf("otlp grpc trace exporter: %w", err)
		}
		metricExporter, err = otlpmetricgrpc.New(ctx, mopts...)
		if err != nil {
			_ = traceExporter.Shutdown(ctx)
			return nil, nil, fmt.Errorf("otlp grpc metric exporter: %w", err)
		}
	case "http":
		topts := otlpHTTPTraceOptions(o.Endpoint, o.Insecure)
		mopts := otlpHTTPMetricOptions(o.Endpoint, o.Insecure)
		traceExporter, err = otlptracehttp.New(ctx, topts...)
		if err != nil {
			return nil, nil, fmt.Errorf("otlp http trace exporter: %w", err)
		}
		metricExporter, err = otlpmetrichttp.New(ctx, mopts...)
		if err != nil {
			_ = traceExporter.Shutdown(ctx)
			return nil, nil, fmt.Errorf("otlp http metric exporter: %w", err)
		}
	default:
		return nil, nil, fmt.Errorf("otlp: unknown protocol %q (use grpc or http)", o.Protocol)
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(traceExporter),
		sdktrace.WithResource(res),
	)
	otel.SetTracerProvider(tp)

	reader := sdkmetric.NewPeriodicReader(metricExporter, sdkmetric.WithInterval(defaultOTLPExportInterval))
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

func otlpGRPCAddr(endpoint string) string {
	e := strings.TrimSpace(endpoint)
	if e == "" {
		return "localhost:4317"
	}
	return stripScheme(e)
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
	return otlpHTTPOptions(endpoint, insecure, func(fullURL string) otlptracehttp.Option {
		return otlptracehttp.WithEndpointURL(fullURL)
	}, func(hostport string) otlptracehttp.Option {
		return otlptracehttp.WithEndpoint(hostport)
	}, func() otlptracehttp.Option {
		return otlptracehttp.WithInsecure()
	})
}

func otlpHTTPMetricOptions(endpoint string, insecure bool) []otlpmetrichttp.Option {
	return otlpHTTPOptions(endpoint, insecure, func(fullURL string) otlpmetrichttp.Option {
		return otlpmetrichttp.WithEndpointURL(fullURL)
	}, func(hostport string) otlpmetrichttp.Option {
		return otlpmetrichttp.WithEndpoint(hostport)
	}, func() otlpmetrichttp.Option {
		return otlpmetrichttp.WithInsecure()
	})
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
