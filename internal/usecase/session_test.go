package usecase_test

import (
	"context"
	"testing"

	"github.com/evrone/go-clean-template/internal/entity"
	"github.com/evrone/go-clean-template/internal/usecase"
	"github.com/evrone/go-clean-template/internal/usecase/session"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"golang.org/x/crypto/bcrypt"
)

func newSessionUseCase(t *testing.T) (usecase.Session, *MockPrincipalRepo) {
	t.Helper()

	repo := NewMockPrincipalRepo(gomock.NewController(t))

	return session.New(repo), repo
}

func TestSession_SignIn(t *testing.T) {
	t.Parallel()

	hash, err := bcrypt.GenerateFromPassword([]byte("right"), bcrypt.MinCost)
	require.NoError(t, err)

	lan := entity.Principal{UserID: "u-1", Email: "lan@acme.test"}

	t.Run("the right password signs in, whatever the email's case", func(t *testing.T) {
		t.Parallel()

		uc, repo := newSessionUseCase(t)
		repo.EXPECT().ByEmail(gomock.Any(), "lan@acme.test").Return(lan, string(hash), nil)

		p, err := uc.SignIn(context.Background(), " Lan@ACME.test ", "right")

		require.NoError(t, err)
		assert.Equal(t, "u-1", p.UserID)
	})

	t.Run("a wrong password and an unknown email are refused alike", func(t *testing.T) {
		t.Parallel()

		uc, repo := newSessionUseCase(t)
		repo.EXPECT().ByEmail(gomock.Any(), "lan@acme.test").Return(lan, string(hash), nil)
		repo.EXPECT().ByEmail(gomock.Any(), "nobody@acme.test").Return(entity.Principal{}, "", entity.ErrUserNotFound)

		_, wrong := uc.SignIn(context.Background(), "lan@acme.test", "wrong")
		_, unknown := uc.SignIn(context.Background(), "nobody@acme.test", "right")

		require.ErrorIs(t, wrong, entity.ErrUnauthorized)
		require.ErrorIs(t, unknown, entity.ErrUnauthorized)
	})

	t.Run("a person since removed is signed out", func(t *testing.T) {
		t.Parallel()

		uc, repo := newSessionUseCase(t)
		repo.EXPECT().ByID(gomock.Any(), "u-gone").Return(entity.Principal{}, entity.ErrUserNotFound)

		_, err := uc.Principal(context.Background(), "u-gone")

		require.ErrorIs(t, err, entity.ErrUnauthorized)
	})
}
