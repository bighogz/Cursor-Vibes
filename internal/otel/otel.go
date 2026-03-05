// Package otel initializes OpenTelemetry tracing. When OTEL_ENABLED=true,
// traces are exported to stdout (structured JSON). In production, swap the
// exporter for OTLP/gRPC pointing at Jaeger, Tempo, or Datadog.
//
// This demonstrates distributed tracing across the Go HTTP layer and into
// Rust (via span context passed through the wasm/subprocess boundary).
package otel

import (
	"context"
	"log"
	"os"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
)

var tracer trace.Tracer

// Init sets up the global tracer provider. Call once from main().
// Returns a shutdown function that flushes pending spans.
func Init(serviceName, version string) func() {
	if os.Getenv("OTEL_ENABLED") != "true" {
		tracer = otel.Tracer(serviceName)
		return func() {}
	}

	exporter, err := stdouttrace.New(stdouttrace.WithPrettyPrint())
	if err != nil {
		log.Printf("otel: exporter init error: %v", err)
		tracer = otel.Tracer(serviceName)
		return func() {}
	}

	res := resource.NewWithAttributes(
		semconv.SchemaURL,
		semconv.ServiceNameKey.String(serviceName),
		semconv.ServiceVersionKey.String(version),
	)

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
	)
	otel.SetTracerProvider(tp)
	tracer = tp.Tracer(serviceName)

	log.Printf("otel: tracing enabled (stdout exporter)")
	return func() {
		if err := tp.Shutdown(context.Background()); err != nil {
			log.Printf("otel: shutdown error: %v", err)
		}
	}
}

// Tracer returns the application's tracer.
func Tracer() trace.Tracer {
	if tracer == nil {
		tracer = otel.Tracer("vibes")
	}
	return tracer
}

// StartSpan is a convenience wrapper for creating a span.
func StartSpan(ctx context.Context, name string) (context.Context, trace.Span) {
	return Tracer().Start(ctx, name)
}
