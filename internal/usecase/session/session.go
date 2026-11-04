// Package session implements who is signed in, for the client's session
// cookie: signing in by email and password, and reading the person back.
package session

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/evrone/go-clean-template/internal/entity"
	"github.com/evrone/go-clean-template/internal/repo"
	"github.com/evrone/go-clean-template/internal/usecase"
	"golang.org/x/crypto/bcrypt"
)

// UseCase -.
type UseCase struct {
	repo repo.PrincipalRepo
}

// New returns a Session usecase instrumented with OpenTelemetry tracing spans.
func New(r repo.PrincipalRepo) usecase.Session {
	return newTraced(&UseCase{repo: r})
}

// SignIn refuses a wrong email and a wrong password alike, so the answer
// never says which emails have an account.
func (uc *UseCase) SignIn(ctx context.Context, email, password string) (entity.Principal, error) {
	p, hash, err := uc.repo.ByEmail(ctx, strings.ToLower(strings.TrimSpace(email)))
	if errors.Is(err, entity.ErrUserNotFound) {
		return entity.Principal{}, entity.ErrUnauthorized
	}

	if err != nil {
		return entity.Principal{}, fmt.Errorf("SessionUseCase - SignIn - uc.repo.ByEmail: %w", err)
	}

	if bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) != nil {
		return entity.Principal{}, entity.ErrUnauthorized
	}

	return p, nil
}

// Principal is the person a session cookie names; one since removed is signed out.
func (uc *UseCase) Principal(ctx context.Context, userID string) (entity.Principal, error) {
	p, err := uc.repo.ByID(ctx, userID)
	if errors.Is(err, entity.ErrUserNotFound) {
		return entity.Principal{}, entity.ErrUnauthorized
	}

	if err != nil {
		return entity.Principal{}, fmt.Errorf("SessionUseCase - Principal - uc.repo.ByID: %w", err)
	}

	return p, nil
}
