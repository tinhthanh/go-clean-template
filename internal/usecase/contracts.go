// Package usecase implements application business logic. Each logic group in own file.
package usecase

import (
	"context"
	"time"

	"github.com/evrone/go-clean-template/internal/entity"
)

//go:generate mockgen -source=contracts.go -destination=./mocks_usecase_test.go -package=usecase_test

type (
	// Translation -.
	Translation interface {
		Translate(ctx context.Context, userID string, t entity.Translation) (entity.Translation, error)
		History(ctx context.Context, userID string) (entity.TranslationHistory, error)
	}

	// User -.
	User interface {
		Register(ctx context.Context, username, email, password string) (entity.User, error)
		Login(ctx context.Context, email, password string) (string, error)
		GetUser(ctx context.Context, userID string) (entity.User, error)
	}

	// Task -.
	Task interface {
		Create(ctx context.Context, userID, title, description string) (entity.Task, error)
		Get(ctx context.Context, userID, taskID string) (entity.Task, error)
		List(ctx context.Context, userID string, status *entity.TaskStatus, limit, offset int) ([]entity.Task, int, error)
		Update(ctx context.Context, userID, taskID, title, description string) (entity.Task, error)
		Transition(ctx context.Context, userID, taskID string, newStatus entity.TaskStatus) (entity.Task, error)
		Delete(ctx context.Context, userID, taskID string) error
	}

	// Session is who is signed in, for the client's session cookie.
	Session interface {
		SignIn(ctx context.Context, email, password string) (entity.Principal, error)
		Principal(ctx context.Context, userID string) (entity.Principal, error)
	}

	// Audit writes the log's lines.
	Audit interface {
		Write(ctx context.Context, line *entity.AuditLine) error
		// WriteOnce writes the line at most once in the window per actor and target.
		WriteOnce(ctx context.Context, line *entity.AuditLine, within time.Duration) error
	}

	// Patient is a clinic's patients, as the person reading may see them.
	Patient interface {
		List(ctx context.Context, p *entity.Principal, q *entity.PatientQuery) (entity.PatientPage, error)
		Get(ctx context.Context, p *entity.Principal, id string) (entity.Patient, error)
		Create(ctx context.Context, p *entity.Principal, in *entity.Patient) (entity.Patient, error)
		// Update answers the patient as changed, and each field's old and new value.
		Update(ctx context.Context, p *entity.Principal, id string, patch *entity.PatientPatch) (entity.Patient, []entity.Change, error)
	}
)
