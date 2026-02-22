package translation

import (
	"context"
	"fmt"

	"github.com/evrone/go-clean-template/internal/entity"
	"github.com/evrone/go-clean-template/internal/repo"
)

const _defaultHistoryLimit = 100

// UseCase -.
type UseCase struct {
	repo   repo.TranslationRepo
	webAPI repo.TranslationWebAPI
}

// New -.
func New(r repo.TranslationRepo, w repo.TranslationWebAPI) *UseCase {
	return &UseCase{
		repo:   r,
		webAPI: w,
	}
}

// History - getting translate history from store.
func (uc *UseCase) History(ctx context.Context) (entity.TranslationHistory, error) {
	translations, err := uc.repo.GetHistory(ctx, _defaultHistoryLimit, 0)
	if err != nil {
		return entity.TranslationHistory{}, fmt.Errorf("TranslationUseCase - History - s.repo.GetHistory: %w", err)
	}

	return entity.TranslationHistory{History: translations}, nil
}

// Translate -.
func (uc *UseCase) Translate(ctx context.Context, t entity.Translation) (entity.Translation, error) {
	translation, err := uc.webAPI.Translate(ctx, t)
	if err != nil {
		return entity.Translation{}, entity.NewAppError(entity.ErrExternalService, fmt.Errorf("TranslationUseCase - Translate - s.webAPI.Translate: %w", err))
	}

	err = uc.repo.Store(ctx, translation)
	if err != nil {
		return entity.Translation{}, entity.NewAppError(entity.ErrInternal, fmt.Errorf("TranslationUseCase - Translate - s.repo.Store: %w", err))
	}

	return translation, nil
}
