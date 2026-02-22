// Package tracer provides OpenTelemetry tracing setup.
package tracer

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
)

// Config holds tracer configuration.
type Config struct {
	ServiceName    string
	ServiceVersion string
	ExporterURL    string
	Environment    string
}

// Tracer wraps the OpenTelemetry TracerProvider.
type Tracer struct {
	Provider *sdktrace.TracerProvider
}

// New creates a new OpenTelemetry tracer with OTLP HTTP exporter.
func New(ctx context.Context, cfg Config) (*Tracer, error) {
	exporter, err := otlptracehttp.New(ctx,
		otlptracehttp.WithEndpointURL(cfg.ExporterURL),
	)
	if err != nil {
		return nil, fmt.Errorf("tracer - New - otlptracehttp.New: %w", err)
	}

	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceName(cfg.ServiceName),
			semconv.ServiceVersion(cfg.ServiceVersion),
			attribute.String("deployment.environment", cfg.Environment),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("tracer - New - resource.New: %w", err)
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
	)

	// Set global tracer provider and propagator.
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	return &Tracer{Provider: tp}, nil
}

// Shutdown gracefully shuts down the tracer provider.
func (t *Tracer) Shutdown(ctx context.Context) error {
	if t.Provider != nil {
		return t.Provider.Shutdown(ctx) //nolint:wrapcheck // passthrough
	}

	return nil
}

// Tracer returns a named tracer from the global provider.
func GetTracer(name string) trace.Tracer {
	return otel.Tracer(name)
}
