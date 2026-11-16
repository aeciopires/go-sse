// Package telemetry configures OpenTelemetry tracing and log export (OTLP/HTTP).
//
// The exporters are configured with the standard OTEL_* environment variables
// (OTEL_EXPORTER_OTLP_ENDPOINT, OTEL_EXPORTER_OTLP_HEADERS, ...), so the same
// binary works with an OpenTelemetry Collector, Grafana Alloy, Tempo, Jaeger
// or any other OTLP-compatible backend.
package telemetry

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"go.opentelemetry.io/contrib/bridges/otelslog"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.43.0"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"
)

// Exporter constructors, replaceable in tests.
var (
	newTraceExporter = func(ctx context.Context) (sdktrace.SpanExporter, error) { return otlptracehttp.New(ctx) }
	newLogExporter   = func(ctx context.Context) (sdklog.Exporter, error) { return otlploghttp.New(ctx) }
)

// Options configures Setup.
type Options struct {
	// Enabled turns OTLP export on. When false, no-op providers are used.
	Enabled        bool
	ServiceName    string
	ServiceVersion string
}

// Telemetry exposes the configured providers.
type Telemetry struct {
	// TracerProvider creates tracers. It is also installed globally.
	TracerProvider trace.TracerProvider
	// LogHandler forwards slog records to the OTLP log exporter.
	// It is nil when telemetry is disabled.
	LogHandler slog.Handler

	shutdown []func(context.Context) error
}

// Setup creates the tracer and logger providers and installs the global
// tracer provider and W3C trace-context propagator.
func Setup(ctx context.Context, opts Options) (*Telemetry, error) {
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{}, propagation.Baggage{},
	))

	if !opts.Enabled {
		tp := noop.NewTracerProvider()
		otel.SetTracerProvider(tp)
		return &Telemetry{TracerProvider: tp}, nil
	}

	res, err := resource.Merge(resource.Default(), resource.NewSchemaless(
		semconv.ServiceName(opts.ServiceName),
		semconv.ServiceVersion(opts.ServiceVersion),
	))
	if err != nil {
		return nil, fmt.Errorf("build otel resource: %w", err)
	}

	traceExp, err := newTraceExporter(ctx)
	if err != nil {
		return nil, fmt.Errorf("create OTLP trace exporter: %w", err)
	}
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(traceExp),
		sdktrace.WithResource(res),
	)

	logExp, err := newLogExporter(ctx)
	if err != nil {
		return nil, errors.Join(fmt.Errorf("create OTLP log exporter: %w", err), tp.Shutdown(ctx))
	}
	lp := sdklog.NewLoggerProvider(
		sdklog.WithProcessor(sdklog.NewBatchProcessor(logExp)),
		sdklog.WithResource(res),
	)

	otel.SetTracerProvider(tp)

	return &Telemetry{
		TracerProvider: tp,
		LogHandler:     otelslog.NewHandler(opts.ServiceName, otelslog.WithLoggerProvider(lp)),
		shutdown:       []func(context.Context) error{tp.Shutdown, lp.Shutdown},
	}, nil
}

// Shutdown flushes pending spans and logs and releases the exporters.
func (t *Telemetry) Shutdown(ctx context.Context) error {
	var errs []error
	for _, fn := range t.shutdown {
		errs = append(errs, fn(ctx))
	}
	return errors.Join(errs...)
}
