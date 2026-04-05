package telemetry

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
)

// InitProvider bootstraps the OpenTelemetry pipeline.
func InitProvider(serviceName string, collectorEndpoint string) (*sdktrace.TracerProvider, error) {
	ctx := context.Background()

	// 1. Configure the HTTP Exporter to send spans to Jaeger
	// Note: We use WithInsecure() because our local Jaeger container doesn't use HTTPS.
	exporter, err := otlptracehttp.New(ctx,
		otlptracehttp.WithEndpoint(collectorEndpoint),
		otlptracehttp.WithInsecure(),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create OTLP exporter: %w", err)
	}

	// 2. Define the resource (Who is sending this data?)
	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceName(serviceName),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	// 3. Create the Tracer Provider
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
	)

	// 4. Set the global defaults
	otel.SetTracerProvider(tp)

	// CRITICAL: Configure the W3C Trace Context Propagator
	// This ensures that TraceIDs are properly extracted/injected when we pass context.
	otel.SetTextMapPropagator(propagation.TraceContext{})

	return tp, nil
}