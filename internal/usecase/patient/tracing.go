package patient

import (
	"context"

	"github.com/evrone/go-clean-template/internal/entity"
	"github.com/evrone/go-clean-template/internal/usecase"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

const _tracerName = "github.com/evrone/go-clean-template/internal/usecase/patient"

// tracedUseCase wraps a Patient usecase with OpenTelemetry spans: ids only,
// never a patient's name, phone or what was searched.
type tracedUseCase struct {
	next usecase.Patient
}

func newTraced(next usecase.Patient) usecase.Patient {
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

func (u *tracedUseCase) List(ctx context.Context, p *entity.Principal, q *entity.PatientQuery) (entity.PatientPage, error) {
	ctx, span := startSpan(ctx, "PatientUseCase.List", attribute.String("user.id", p.UserID))

	result, err := u.next.List(ctx, p, q)
	endSpan(span, err)

	return result, err
}

func (u *tracedUseCase) Get(ctx context.Context, p *entity.Principal, id string) (entity.Patient, error) {
	ctx, span := startSpan(ctx, "PatientUseCase.Get", attribute.String("user.id", p.UserID), attribute.String("patient.id", id))

	result, err := u.next.Get(ctx, p, id)
	endSpan(span, err)

	return result, err
}

func (u *tracedUseCase) Create(ctx context.Context, p *entity.Principal, in *entity.Patient) (entity.Patient, error) {
	ctx, span := startSpan(ctx, "PatientUseCase.Create", attribute.String("user.id", p.UserID))

	result, err := u.next.Create(ctx, p, in)
	endSpan(span, err)

	return result, err
}

func (u *tracedUseCase) Update(ctx context.Context, p *entity.Principal, id string, patch *entity.PatientPatch) (entity.Patient, []entity.Change, error) {
	ctx, span := startSpan(ctx, "PatientUseCase.Update", attribute.String("user.id", p.UserID), attribute.String("patient.id", id))

	result, changes, err := u.next.Update(ctx, p, id, patch)
	endSpan(span, err)

	return result, changes, err
}
