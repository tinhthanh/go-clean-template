package usecase_test

import (
	"context"
	"testing"

	"github.com/evrone/go-clean-template/internal/entity"
	"github.com/evrone/go-clean-template/internal/usecase"
	"github.com/evrone/go-clean-template/internal/usecase/patient"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func newPatientUseCase(t *testing.T) (usecase.Patient, *MockPatientRepo) {
	t.Helper()

	repo := NewMockPatientRepo(gomock.NewController(t))

	return patient.New(repo), repo
}

// desk works at CN1 and sees phones masked; owner works everywhere and sees them whole.
func desk() *entity.Principal {
	return &entity.Principal{UserID: "u-desk", Workspaces: []string{"b-cn1"}}
}

func owner() *entity.Principal {
	return &entity.Principal{UserID: "u-owner", Grants: []string{"patient.phone.view_full"}}
}

//go:fix inline
func TestPatient_List(t *testing.T) {
	t.Parallel()

	t.Run("another branch's patients are not the person's to list", func(t *testing.T) {
		t.Parallel()

		uc, _ := newPatientUseCase(t)

		_, err := uc.List(context.Background(), desk(), &entity.PatientQuery{MadeAt: []string{"b-cn2"}})

		require.ErrorIs(t, err, entity.ErrNotMember)
	})

	t.Run("a reader who sees phones masked finds a number by its last three only", func(t *testing.T) {
		t.Parallel()

		uc, repo := newPatientUseCase(t)
		repo.EXPECT().List(gomock.Any(), gomock.Cond(func(q *entity.PatientQuery) bool { return q.PhoneDigitsOnly })).Return(entity.PatientPage{}, nil)
		repo.EXPECT().List(gomock.Any(), gomock.Cond(func(q *entity.PatientQuery) bool { return !q.PhoneDigitsOnly })).Return(entity.PatientPage{}, nil)

		_, err := uc.List(context.Background(), desk(), &entity.PatientQuery{Q: "152"})
		require.NoError(t, err)

		_, err = uc.List(context.Background(), owner(), &entity.PatientQuery{Q: "991"})
		require.NoError(t, err)
	})

	t.Run("a page asks for twenty unless told, and never more than two hundred", func(t *testing.T) {
		t.Parallel()

		uc, repo := newPatientUseCase(t)
		repo.EXPECT().List(gomock.Any(), gomock.Cond(func(q *entity.PatientQuery) bool { return q.Limit == 20 })).Return(entity.PatientPage{}, nil)

		_, err := uc.List(context.Background(), owner(), &entity.PatientQuery{})
		require.NoError(t, err)

		_, err = uc.List(context.Background(), owner(), &entity.PatientQuery{Limit: 201})

		var field *entity.FieldError
		require.ErrorAs(t, err, &field)
		assert.Equal(t, "limit", field.Field)
	})

	t.Run("a sort the list doesn't have is refused on its field", func(t *testing.T) {
		t.Parallel()

		uc, _ := newPatientUseCase(t)

		_, err := uc.List(context.Background(), owner(), &entity.PatientQuery{Sort: "spent"})

		var field *entity.FieldError
		require.ErrorAs(t, err, &field)
		assert.Equal(t, "sort", field.Field)
	})
}

func TestPatient_Create(t *testing.T) {
	t.Parallel()

	t.Run("a record is made at one of the person's branches only", func(t *testing.T) {
		t.Parallel()

		uc, _ := newPatientUseCase(t)

		_, err := uc.Create(context.Background(), desk(), &entity.Patient{Workspace: "b-cn2", Name: "An", Phone: "0901234567"})

		require.ErrorIs(t, err, entity.ErrNotMember)
	})

	t.Run("each refused field is named", func(t *testing.T) {
		t.Parallel()

		uc, _ := newPatientUseCase(t)

		for field, in := range map[string]entity.Patient{
			"workspace": {Name: "An", Phone: "0901234567"},
			"name":      {Workspace: "b-cn1", Name: "  ", Phone: "0901234567"},
			"phone":     {Workspace: "b-cn1", Name: "An", Phone: "0901"},
			"gender":    {Workspace: "b-cn1", Name: "An", Phone: "0901234567", Gender: "other"},
			"birth":     {Workspace: "b-cn1", Name: "An", Phone: "0901234567", Birth: "1990-02-30"},
		} {
			_, err := uc.Create(context.Background(), desk(), &in)

			var got *entity.FieldError
			require.ErrorAs(t, err, &got, field)
			assert.Equal(t, field, got.Field)
		}
	})

	t.Run("a valid record is stored, its name trimmed", func(t *testing.T) {
		t.Parallel()

		uc, repo := newPatientUseCase(t)
		repo.EXPECT().Store(gomock.Any(), gomock.Cond(func(p *entity.Patient) bool { return p.Name == "An" })).Return(nil)

		_, err := uc.Create(context.Background(), desk(), &entity.Patient{Workspace: "b-cn1", Name: " An ", Phone: "0901 234 567"})

		require.NoError(t, err)
	})
}

func TestPatient_Update(t *testing.T) {
	t.Parallel()

	stored := entity.Patient{ID: "p-1", Workspace: "b-cn1", Name: "An", Phone: "0901234567"}

	t.Run("a reader who sees the phone masked may not change it", func(t *testing.T) {
		t.Parallel()

		uc, _ := newPatientUseCase(t)

		_, _, err := uc.Update(context.Background(), desk(), "p-1", &entity.PatientPatch{Phone: new("0907654321")})

		require.ErrorIs(t, err, entity.ErrForbidden)
	})

	t.Run("each field changed is answered with its old and new value", func(t *testing.T) {
		t.Parallel()

		uc, repo := newPatientUseCase(t)
		repo.EXPECT().Get(gomock.Any(), "p-1").Return(stored, nil)
		repo.EXPECT().Update(gomock.Any(), gomock.Any()).Return(nil)

		p, changes, err := uc.Update(context.Background(), owner(), "p-1", &entity.PatientPatch{Name: new("An Nguyễn"), Note: new("đau răng"), Phone: new("0901234567")})

		require.NoError(t, err)
		assert.Equal(t, "An Nguyễn", p.Name)
		assert.Equal(t, []entity.Change{{Field: "name", From: "An", To: "An Nguyễn"}, {Field: "note", From: "", To: "đau răng"}}, changes, "the phone, sent unchanged, is no change")
	})

	t.Run("nothing changed writes nothing", func(t *testing.T) {
		t.Parallel()

		uc, repo := newPatientUseCase(t)
		repo.EXPECT().Get(gomock.Any(), "p-1").Return(stored, nil)

		_, changes, err := uc.Update(context.Background(), desk(), "p-1", &entity.PatientPatch{Name: new("An")})

		require.NoError(t, err)
		assert.Empty(t, changes)
	})
}
