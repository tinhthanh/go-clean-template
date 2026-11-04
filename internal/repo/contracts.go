// Package repo implements application outer layer logic. Each logic group in own file.
package repo

import (
	"context"
	"time"

	"github.com/evrone/go-clean-template/internal/entity"
)

//go:generate mockgen -source=contracts.go -destination=../usecase/mocks_repo_test.go -package=usecase_test

type (
	// TranslationRepo -.
	TranslationRepo interface {
		Store(ctx context.Context, userID string, t entity.Translation) error
		GetHistory(ctx context.Context, userID string) ([]entity.Translation, error)
	}

	// TranslationWebAPI -.
	TranslationWebAPI interface {
		Translate(ctx context.Context, t entity.Translation) (entity.Translation, error)
	}

	// UserRepo -.
	UserRepo interface {
		Store(ctx context.Context, user *entity.User) error
		GetByID(ctx context.Context, id string) (entity.User, error)
		GetByEmail(ctx context.Context, email string) (entity.User, error)
	}

	// TaskRepo -.
	TaskRepo interface {
		Store(ctx context.Context, task *entity.Task) error
		GetByID(ctx context.Context, userID, taskID string) (entity.Task, error)
		List(ctx context.Context, userID string, filter TaskFilter) ([]entity.Task, int, error)
		Update(ctx context.Context, task *entity.Task) error
		Delete(ctx context.Context, userID, taskID string) error
	}

	// PrincipalRepo reads who a person is: their group's grants and their workspaces.
	PrincipalRepo interface {
		ByID(ctx context.Context, userID string) (entity.Principal, error)
		// ByEmail answers the person and their password's hash, for signing in.
		ByEmail(ctx context.Context, email string) (entity.Principal, string, error)
	}

	// AuditRepo adds lines to the log, which nothing changes or removes.
	AuditRepo interface {
		Write(ctx context.Context, line *entity.AuditLine) error
		// WriteOnce writes the line unless the same actor's same line on the
		// same target was written within the window; it answers whether it wrote.
		WriteOnce(ctx context.Context, line *entity.AuditLine, within time.Duration) (bool, error)
	}

	// PatientRepo -.
	PatientRepo interface {
		List(ctx context.Context, q *entity.PatientQuery) (entity.PatientPage, error)
		Get(ctx context.Context, id string) (entity.Patient, error)
		// Store issues the patient's code and id.
		Store(ctx context.Context, p *entity.Patient) error
		Update(ctx context.Context, p *entity.Patient) error
	}

	// TaskFilter -.
	TaskFilter struct {
		Status *entity.TaskStatus
		Limit  uint64
		Offset uint64
	}
)
