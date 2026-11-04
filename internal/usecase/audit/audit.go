// Package audit implements writing the log's lines.
package audit

import (
	"context"
	"fmt"
	"time"

	"github.com/evrone/go-clean-template/internal/entity"
	"github.com/evrone/go-clean-template/internal/repo"
	"github.com/evrone/go-clean-template/internal/usecase"
)

// UseCase -.
type UseCase struct {
	repo repo.AuditRepo
}

// New returns an Audit usecase instrumented with OpenTelemetry tracing spans.
func New(r repo.AuditRepo) usecase.Audit {
	return newTraced(&UseCase{repo: r})
}

// Write -.
func (uc *UseCase) Write(ctx context.Context, line *entity.AuditLine) error {
	if err := uc.repo.Write(ctx, line); err != nil {
		return fmt.Errorf("AuditUseCase - Write - uc.repo.Write: %w", err)
	}

	return nil
}

// WriteOnce -.
func (uc *UseCase) WriteOnce(ctx context.Context, line *entity.AuditLine, within time.Duration) error {
	if _, err := uc.repo.WriteOnce(ctx, line, within); err != nil {
		return fmt.Errorf("AuditUseCase - WriteOnce - uc.repo.WriteOnce: %w", err)
	}

	return nil
}
