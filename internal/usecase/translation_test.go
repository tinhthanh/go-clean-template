package usecase_test

import (
	"context"
	"errors"
	"testing"

	"github.com/evrone/go-clean-template/internal/entity"
	"github.com/evrone/go-clean-template/internal/usecase/translation"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

var errInternalServErr = errors.New("internal server error")

type test struct {
	name string
	mock func()
	res  interface{}
	err  bool
}

func translationUseCase(t *testing.T) (*translation.UseCase, *MockTranslationRepo, *MockTranslationWebAPI) {
	t.Helper()

	mockCtl := gomock.NewController(t)
	defer mockCtl.Finish()

	repo := NewMockTranslationRepo(mockCtl)
	webAPI := NewMockTranslationWebAPI(mockCtl)

	useCase := translation.New(repo, webAPI)

	return useCase, repo, webAPI
}

func TestHistory(t *testing.T) { //nolint:tparallel // data races here
	t.Parallel()

	translationUseCase, repo, _ := translationUseCase(t)

	tests := []test{
		{
			name: "empty result",
			mock: func() {
				repo.EXPECT().GetHistory(context.Background(), 100, 0).Return(nil, nil)
			},
			res: entity.TranslationHistory{},
			err: false,
		},
		{
			name: "result with error",
			mock: func() {
				repo.EXPECT().GetHistory(context.Background(), 100, 0).Return(nil, errInternalServErr)
			},
			res: entity.TranslationHistory{},
			err: true,
		},
		{
			name: "success with data",
			mock: func() {
				repo.EXPECT().GetHistory(context.Background(), 100, 0).Return([]entity.Translation{
					{
						Source:      "en",
						Destination: "vi",
						Original:    "hello",
						Translation: "xin chào",
					},
					{
						Source:      "vi",
						Destination: "en",
						Original:    "xin chào",
						Translation: "hello",
					},
				}, nil)
			},
			res: entity.TranslationHistory{
				History: []entity.Translation{
					{
						Source:      "en",
						Destination: "vi",
						Original:    "hello",
						Translation: "xin chào",
					},
					{
						Source:      "vi",
						Destination: "en",
						Original:    "xin chào",
						Translation: "hello",
					},
				},
			},
			err: false,
		},
	}

	for _, tc := range tests { //nolint:paralleltest // data races here
		localTc := tc

		t.Run(localTc.name, func(t *testing.T) {
			localTc.mock()

			res, err := translationUseCase.History(context.Background())

			require.Equal(t, localTc.res, res)

			if localTc.err {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestTranslate(t *testing.T) { //nolint:tparallel // data races here
	t.Parallel()

	translationUseCase, repo, webAPI := translationUseCase(t)

	inputTranslation := entity.Translation{
		Source:      "en",
		Destination: "vi",
		Original:    "hello world",
	}

	translatedResult := entity.Translation{
		Source:      "en",
		Destination: "vi",
		Original:    "hello world",
		Translation: "xin chào thế giới",
	}

	tests := []test{
		{
			name: "success",
			mock: func() {
				webAPI.EXPECT().Translate(context.Background(), inputTranslation).Return(translatedResult, nil)
				repo.EXPECT().Store(context.Background(), translatedResult).Return(nil)
			},
			res: translatedResult,
			err: false,
		},
		{
			name: "web API error",
			mock: func() {
				webAPI.EXPECT().Translate(context.Background(), inputTranslation).Return(entity.Translation{}, errInternalServErr)
			},
			res: entity.Translation{},
			err: true,
		},
		{
			name: "repo store error",
			mock: func() {
				webAPI.EXPECT().Translate(context.Background(), inputTranslation).Return(translatedResult, nil)
				repo.EXPECT().Store(context.Background(), translatedResult).Return(errInternalServErr)
			},
			res: entity.Translation{},
			err: true,
		},
		{
			name: "empty input translation",
			mock: func() {
				webAPI.EXPECT().Translate(context.Background(), entity.Translation{}).Return(entity.Translation{}, nil)
				repo.EXPECT().Store(context.Background(), entity.Translation{}).Return(nil)
			},
			res: entity.Translation{},
			err: false,
		},
	}

	for _, tc := range tests { //nolint:paralleltest // data races here
		localTc := tc

		t.Run(localTc.name, func(t *testing.T) {
			localTc.mock()

			var res entity.Translation

			var err error

			if localTc.name == "empty input translation" {
				res, err = translationUseCase.Translate(context.Background(), entity.Translation{})
			} else {
				res, err = translationUseCase.Translate(context.Background(), inputTranslation)
			}

			require.EqualValues(t, localTc.res, res)

			if localTc.err {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestTranslateAppError(t *testing.T) {
	t.Parallel()

	translationUseCase, _, webAPI := translationUseCase(t)

	t.Run("web API error wraps ErrExternalService", func(t *testing.T) {
		webAPI.EXPECT().Translate(gomock.Any(), gomock.Any()).Return(entity.Translation{}, errInternalServErr)

		_, err := translationUseCase.Translate(context.Background(), entity.Translation{})

		require.Error(t, err)

		appErr := entity.GetAppError(err)
		require.Equal(t, entity.ErrExternalService.Code, appErr.Code)
	})
}

func TestTranslateRepoAppError(t *testing.T) {
	t.Parallel()

	translationUseCase, repo, webAPI := translationUseCase(t)

	t.Run("repo error wraps ErrInternal", func(t *testing.T) {
		webAPI.EXPECT().Translate(gomock.Any(), gomock.Any()).Return(entity.Translation{}, nil)
		repo.EXPECT().Store(gomock.Any(), gomock.Any()).Return(errInternalServErr)

		_, err := translationUseCase.Translate(context.Background(), entity.Translation{})

		require.Error(t, err)

		appErr := entity.GetAppError(err)
		require.Equal(t, entity.ErrInternal.Code, appErr.Code)
	})
}
