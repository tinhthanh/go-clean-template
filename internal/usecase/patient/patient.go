// Package patient implements a clinic's patients as the person reading may
// see them: which branches a list covers, what a phone search may match,
// and who may change a phone number. What a reader may not see of a row is
// the contract's x-masks, applied by the REST layer.
package patient

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/evrone/go-clean-template/internal/entity"
	"github.com/evrone/go-clean-template/internal/repo"
	"github.com/evrone/go-clean-template/internal/usecase"
)

const (
	_defaultLimit = 20
	// The most a page may ask for (NFR-03): the client is told, not cut short silently.
	_maxLimit       = 200
	_minPhoneDigits = 9
	_maxPhoneDigits = 11
	_phoneFull      = "patient.phone.view_full"
)

// UseCase -.
type UseCase struct {
	repo repo.PatientRepo
}

// New returns a Patient usecase instrumented with OpenTelemetry tracing spans.
func New(r repo.PatientRepo) usecase.Patient {
	return newTraced(&UseCase{repo: r})
}

// List covers the branches asked of madeAt that are the person's; one that
// isn't is refused. A reader who sees phones masked finds a number only by
// its last three digits, as the mask shows them.
func (uc *UseCase) List(ctx context.Context, p *entity.Principal, q *entity.PatientQuery) (entity.PatientPage, error) {
	for _, w := range q.MadeAt {
		if !p.Works(w) {
			return entity.PatientPage{}, entity.ErrNotMember
		}
	}

	if q.Sort != "" && q.Sort != "code" && q.Sort != "name" {
		return entity.PatientPage{}, entity.Invalid("sort", "A list sorts by code or name.")
	}

	switch {
	case q.Limit == 0:
		q.Limit = _defaultLimit
	case q.Limit > _maxLimit:
		return entity.PatientPage{}, entity.Invalid("limit", fmt.Sprintf("A page holds at most %d.", _maxLimit))
	}

	q.PhoneDigitsOnly = !p.Can(_phoneFull)

	page, err := uc.repo.List(ctx, q)
	if err != nil {
		return entity.PatientPage{}, fmt.Errorf("PatientUseCase - List - uc.repo.List: %w", err)
	}

	return page, nil
}

// Get -.
func (uc *UseCase) Get(ctx context.Context, _ *entity.Principal, id string) (entity.Patient, error) {
	pat, err := uc.repo.Get(ctx, id)
	if err != nil {
		return entity.Patient{}, fmt.Errorf("PatientUseCase - Get - uc.repo.Get: %w", err)
	}

	return pat, nil
}

// Create makes the record at a branch of the person's.
func (uc *UseCase) Create(ctx context.Context, p *entity.Principal, in *entity.Patient) (entity.Patient, error) {
	if in.Workspace == "" {
		return entity.Patient{}, entity.Invalid("workspace", "Choose the branch the patient is registered at.")
	}

	if !p.Works(in.Workspace) {
		return entity.Patient{}, entity.ErrNotMember
	}

	pat := *in
	pat.Name = strings.TrimSpace(pat.Name)

	if err := valid(&pat); err != nil {
		return entity.Patient{}, err
	}

	if err := uc.repo.Store(ctx, &pat); err != nil {
		return entity.Patient{}, fmt.Errorf("PatientUseCase - Create - uc.repo.Store: %w", err)
	}

	return pat, nil
}

// Update changes the fields sent, and answers each one's old and new value
// for the log. Changing a phone number needs patient.phone.view_full: a
// reader who sees it masked can't know what they would overwrite.
func (uc *UseCase) Update(ctx context.Context, p *entity.Principal, id string, patch *entity.PatientPatch) (entity.Patient, []entity.Change, error) {
	if patch.Phone != nil && !p.Can(_phoneFull) {
		return entity.Patient{}, nil, entity.ErrForbidden
	}

	pat, err := uc.repo.Get(ctx, id)
	if err != nil {
		return entity.Patient{}, nil, fmt.Errorf("PatientUseCase - Update - uc.repo.Get: %w", err)
	}

	changes := apply(&pat, patch)
	if len(changes) == 0 {
		return pat, nil, nil
	}

	if validErr := valid(&pat); validErr != nil {
		return entity.Patient{}, nil, validErr
	}

	if err := uc.repo.Update(ctx, &pat); err != nil {
		return entity.Patient{}, nil, fmt.Errorf("PatientUseCase - Update - uc.repo.Update: %w", err)
	}

	return pat, changes, nil
}

// apply writes the patch's fields over the patient and answers what changed.
func apply(pat *entity.Patient, patch *entity.PatientPatch) []entity.Change {
	fields := []struct {
		name  string
		to    *string
		field *string
	}{
		{"name", patch.Name, &pat.Name},
		{"phone", patch.Phone, &pat.Phone},
		{"gender", patch.Gender, &pat.Gender},
		{"birth", patch.Birth, &pat.Birth},
		{"cccd", patch.CCCD, &pat.CCCD},
		{"email", patch.Email, &pat.Email},
		{"address", patch.Address, &pat.Address},
		{"occupation", patch.Occupation, &pat.Occupation},
		{"note", patch.Note, &pat.Note},
	}

	var changes []entity.Change

	for _, f := range fields {
		if f.to == nil || *f.to == *f.field {
			continue
		}

		changes = append(changes, entity.Change{Field: f.name, From: *f.field, To: *f.to})
		*f.field = *f.to
	}

	return changes
}

// valid holds a record to what the form says: a name, a phone of 9 to 11
// digits, a gender the contract names and a real day of birth.
func valid(pat *entity.Patient) error {
	if strings.TrimSpace(pat.Name) == "" {
		return entity.Invalid("name", "A patient needs a name.")
	}

	digits := len(strings.Map(keepDigit, pat.Phone))
	if digits < _minPhoneDigits || digits > _maxPhoneDigits {
		return entity.Invalid("phone", "A phone number has 9 to 11 digits.")
	}

	if pat.Gender != "" && !slices.Contains([]string{"male", "female"}, pat.Gender) {
		return entity.Invalid("gender", "Choose male or female.")
	}

	if pat.Birth != "" {
		if _, err := time.Parse(time.DateOnly, pat.Birth); err != nil {
			return entity.Invalid("birth", "A day of birth is a real day.")
		}
	}

	return nil
}

func keepDigit(r rune) rune {
	if r >= '0' && r <= '9' {
		return r
	}

	return -1
}
