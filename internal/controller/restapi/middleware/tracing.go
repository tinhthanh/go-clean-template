package middleware

import (
	"fmt"

	"github.com/evrone/go-clean-template/pkg/logger"
	"github.com/gofiber/fiber/v2"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/propagation"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
)

// Tracing returns an OpenTelemetry tracing middleware for Fiber.
func Tracing(serviceName string, l logger.Interface) func(c *fiber.Ctx) error {
	tracer := otel.Tracer(serviceName)

	return func(ctx *fiber.Ctx) error {
		// Extract propagated context from incoming request headers.
		carrier := make(propagation.HeaderCarrier)
		ctx.Request().Header.VisitAll(func(key, value []byte) {
			carrier.Set(string(key), string(value))
		})

		parentCtx := otel.GetTextMapPropagator().Extract(ctx.UserContext(), carrier)

		// Start a new span.
		spanName := fmt.Sprintf("%s %s", ctx.Method(), ctx.Route().Path)

		spanCtx, span := tracer.Start(parentCtx, spanName,
			trace.WithSpanKind(trace.SpanKindServer),
			trace.WithAttributes(
				semconv.HTTPRequestMethodKey.String(ctx.Method()),
				semconv.URLPath(ctx.Path()),
				attribute.String("http.route", ctx.Route().Path),
				attribute.String("net.peer.ip", ctx.IP()),
			),
		)
		defer span.End()

		// Inject span context into fiber context for downstream use.
		ctx.SetUserContext(spanCtx)

		// Set trace ID in response header for correlation.
		if span.SpanContext().HasTraceID() {
			ctx.Set("X-Trace-ID", span.SpanContext().TraceID().String())
		}

		err := ctx.Next()

		// Record response status.
		span.SetAttributes(
			semconv.HTTPResponseStatusCode(ctx.Response().StatusCode()),
		)

		if err != nil {
			span.RecordError(err)
		}

		return err
	}
}
