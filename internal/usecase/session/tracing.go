package session

import (
	"context"

	"github.com/evrone/go-clean-template/internal/entity"
	"github.com/evrone/go-clean-template/internal/usecase"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

const _tracerName = "github.com/evrone/go-clean-template/internal/usecase/session"

// tracedUseCase wraps a Session usecase with OpenTelemetry spans.
type tracedUseCase struct {
	next usecase.Session
}

func newTraced(next usecase.Session) usecase.Session {
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

// SignIn carries no attribute: the email is a person's.
func (u *tracedUseCase) SignIn(ctx context.Context, email, password string) (entity.Principal, error) {
	ctx, span := startSpan(ctx, "SessionUseCase.SignIn")

	result, err := u.next.SignIn(ctx, email, password)
	endSpan(span, err)

	return result, err
}

func (u *tracedUseCase) Principal(ctx context.Context, userID string) (entity.Principal, error) {
	ctx, span := startSpan(ctx, "SessionUseCase.Principal", attribute.String("user.id", userID))

	result, err := u.next.Principal(ctx, userID)
	endSpan(span, err)

	return result, err
}
