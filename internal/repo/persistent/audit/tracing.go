package audit

import (
	"context"
	"time"

	"github.com/evrone/go-clean-template/internal/entity"
	"github.com/evrone/go-clean-template/internal/repo"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

const _tracerName = "github.com/evrone/go-clean-template/internal/repo/persistent/audit"

// tracedRepo wraps an AuditRepo with OpenTelemetry spans.
type tracedRepo struct {
	next repo.AuditRepo
}

func newTraced(next repo.AuditRepo) repo.AuditRepo {
	return &tracedRepo{next: next}
}

func startSpan(ctx context.Context, name string, attrs ...attribute.KeyValue) (context.Context, trace.Span) {
	return otel.Tracer(_tracerName).Start(ctx, name, trace.WithAttributes(attrs...))
}

func endSpan(span trace.Span, err error) {
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}

	span.End()
}

func lineAttrs(line *entity.AuditLine) []attribute.KeyValue {
	return []attribute.KeyValue{attribute.String("audit.action", line.Action), attribute.String("audit.resource", line.Resource)}
}

func (r *tracedRepo) Write(ctx context.Context, line *entity.AuditLine) error {
	ctx, span := startSpan(ctx, "AuditRepo.Write", lineAttrs(line)...)

	err := r.next.Write(ctx, line)
	endSpan(span, err)

	return err
}

func (r *tracedRepo) WriteOnce(ctx context.Context, line *entity.AuditLine, within time.Duration) (bool, error) {
	ctx, span := startSpan(ctx, "AuditRepo.WriteOnce", lineAttrs(line)...)

	wrote, err := r.next.WriteOnce(ctx, line, within)
	endSpan(span, err)

	return wrote, err
}
