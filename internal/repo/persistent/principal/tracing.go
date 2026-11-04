package principal

import (
	"context"

	"github.com/evrone/go-clean-template/internal/entity"
	"github.com/evrone/go-clean-template/internal/repo"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

const _tracerName = "github.com/evrone/go-clean-template/internal/repo/persistent/principal"

// tracedRepo wraps a PrincipalRepo with OpenTelemetry spans.
type tracedRepo struct {
	next repo.PrincipalRepo
}

func newTraced(next repo.PrincipalRepo) repo.PrincipalRepo {
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

func (r *tracedRepo) ByID(ctx context.Context, userID string) (entity.Principal, error) {
	ctx, span := startSpan(ctx, "PrincipalRepo.ByID", attribute.String("user.id", userID))

	result, err := r.next.ByID(ctx, userID)
	endSpan(span, err)

	return result, err
}

// ByEmail carries no attribute: an email is a person's, and spans leave the server.
func (r *tracedRepo) ByEmail(ctx context.Context, email string) (entity.Principal, string, error) {
	ctx, span := startSpan(ctx, "PrincipalRepo.ByEmail")

	result, hash, err := r.next.ByEmail(ctx, email)
	endSpan(span, err)

	return result, hash, err
}
