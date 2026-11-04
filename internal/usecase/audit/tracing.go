package audit

import (
	"context"
	"time"

	"github.com/evrone/go-clean-template/internal/entity"
	"github.com/evrone/go-clean-template/internal/usecase"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

const _tracerName = "github.com/evrone/go-clean-template/internal/usecase/audit"

// tracedUseCase wraps an Audit usecase with OpenTelemetry spans.
type tracedUseCase struct {
	next usecase.Audit
}

func newTraced(next usecase.Audit) usecase.Audit {
	return &tracedUseCase{next: next}
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

func (u *tracedUseCase) Write(ctx context.Context, line *entity.AuditLine) error {
	ctx, span := startSpan(ctx, "AuditUseCase.Write", attribute.String("audit.action", line.Action))

	err := u.next.Write(ctx, line)
	endSpan(span, err)

	return err
}

func (u *tracedUseCase) WriteOnce(ctx context.Context, line *entity.AuditLine, within time.Duration) error {
	ctx, span := startSpan(ctx, "AuditUseCase.WriteOnce", attribute.String("audit.action", line.Action))

	err := u.next.WriteOnce(ctx, line, within)
	endSpan(span, err)

	return err
}
