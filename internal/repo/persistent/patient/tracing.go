package patient

import (
	"context"

	"github.com/evrone/go-clean-template/internal/entity"
	"github.com/evrone/go-clean-template/internal/repo"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

const _tracerName = "github.com/evrone/go-clean-template/internal/repo/persistent/patient"

// tracedRepo wraps a PatientRepo with OpenTelemetry spans. A span carries
// ids only: a patient's name, phone or search words never leave the server.
type tracedRepo struct {
	next repo.PatientRepo
}

func newTraced(next repo.PatientRepo) repo.PatientRepo {
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

func (r *tracedRepo) List(ctx context.Context, q *entity.PatientQuery) (entity.PatientPage, error) {
	ctx, span := startSpan(ctx, "PatientRepo.List", attribute.Int64("patient.limit", int64(q.Limit))) //nolint:gosec // a page size, never near overflow

	result, err := r.next.List(ctx, q)
	endSpan(span, err)

	return result, err
}

func (r *tracedRepo) Get(ctx context.Context, id string) (entity.Patient, error) {
	ctx, span := startSpan(ctx, "PatientRepo.Get", attribute.String("patient.id", id))

	result, err := r.next.Get(ctx, id)
	endSpan(span, err)

	return result, err
}

func (r *tracedRepo) Store(ctx context.Context, p *entity.Patient) error {
	ctx, span := startSpan(ctx, "PatientRepo.Store")

	err := r.next.Store(ctx, p)
	endSpan(span, err)

	return err
}

func (r *tracedRepo) Update(ctx context.Context, p *entity.Patient) error {
	ctx, span := startSpan(ctx, "PatientRepo.Update", attribute.String("patient.id", p.ID))

	err := r.next.Update(ctx, p)
	endSpan(span, err)

	return err
}
