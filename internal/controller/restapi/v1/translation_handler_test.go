package v1_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/evrone/go-clean-template/internal/controller/restapi/middleware"
	v1 "github.com/evrone/go-clean-template/internal/controller/restapi/v1"
	"github.com/evrone/go-clean-template/internal/entity"
	"github.com/evrone/go-clean-template/internal/usecase"
	"github.com/evrone/go-clean-template/pkg/logger"
	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

//go:generate mockgen -source=../../../usecase/contracts.go -destination=./mock_usecase_test.go -package=v1_test

func setupRouter(t *testing.T) (*fiber.App, *MockTranslation) {
	t.Helper()

	mockCtl := gomock.NewController(t)
	defer mockCtl.Finish()

	mockTranslation := NewMockTranslation(mockCtl)
	l := logger.New("error")

	app := fiber.New()
	app.Use(middleware.RequestID())

	group := app.Group("/v1")
	v1.NewTranslationRoutes(group, mockTranslation, l)

	return app, mockTranslation
}

func TestHistoryHandler_Success(t *testing.T) {
	t.Parallel()

	app, mockUseCase := setupRouter(t)

	expectedHistory := entity.TranslationHistory{
		History: []entity.Translation{
			{
				Source:      "en",
				Destination: "vi",
				Original:    "hello",
				Translation: "xin chào",
			},
		},
	}

	mockUseCase.EXPECT().History(gomock.Any()).Return(expectedHistory, nil)

	req := httptest.NewRequest(http.MethodGet, "/v1/translation/history", nil)
	resp, err := app.Test(req)

	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var result entity.TranslationHistory
	err = json.NewDecoder(resp.Body).Decode(&result)

	require.NoError(t, err)
	require.Len(t, result.History, 1)
	require.Equal(t, "xin chào", result.History[0].Translation)
}

func TestHistoryHandler_Error(t *testing.T) {
	t.Parallel()

	app, mockUseCase := setupRouter(t)

	mockUseCase.EXPECT().History(gomock.Any()).Return(entity.TranslationHistory{}, errors.New("db error"))

	req := httptest.NewRequest(http.MethodGet, "/v1/translation/history", nil)
	resp, err := app.Test(req)

	require.NoError(t, err)
	require.Equal(t, http.StatusInternalServerError, resp.StatusCode)
}

func TestDoTranslateHandler_Success(t *testing.T) {
	t.Parallel()

	app, mockUseCase := setupRouter(t)

	expectedTranslation := entity.Translation{
		Source:      "en",
		Destination: "vi",
		Original:    "hello world",
		Translation: "xin chào thế giới",
	}

	mockUseCase.EXPECT().Translate(gomock.Any(), entity.Translation{
		Source:      "en",
		Destination: "vi",
		Original:    "hello world",
	}).Return(expectedTranslation, nil)

	body, _ := json.Marshal(map[string]string{
		"source":      "en",
		"destination": "vi",
		"original":    "hello world",
	})

	req := httptest.NewRequest(http.MethodPost, "/v1/translation/do-translate", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)

	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var result entity.Translation
	err = json.NewDecoder(resp.Body).Decode(&result)

	require.NoError(t, err)
	require.Equal(t, "xin chào thế giới", result.Translation)
}

func TestDoTranslateHandler_InvalidBody(t *testing.T) {
	t.Parallel()

	app, _ := setupRouter(t)

	req := httptest.NewRequest(http.MethodPost, "/v1/translation/do-translate", bytes.NewReader([]byte("invalid json")))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)

	require.NoError(t, err)
	require.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestDoTranslateHandler_ValidationError(t *testing.T) {
	t.Parallel()

	app, _ := setupRouter(t)

	// Missing required fields.
	body, _ := json.Marshal(map[string]string{
		"source": "en",
	})

	req := httptest.NewRequest(http.MethodPost, "/v1/translation/do-translate", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)

	require.NoError(t, err)
	require.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestDoTranslateHandler_ServiceError(t *testing.T) {
	t.Parallel()

	app, mockUseCase := setupRouter(t)

	mockUseCase.EXPECT().Translate(gomock.Any(), gomock.Any()).Return(
		entity.Translation{},
		entity.NewAppError(entity.ErrExternalService, errors.New("google translate unavailable")),
	)

	body, _ := json.Marshal(map[string]string{
		"source":      "en",
		"destination": "vi",
		"original":    "hello",
	})

	req := httptest.NewRequest(http.MethodPost, "/v1/translation/do-translate", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)

	require.NoError(t, err)
	require.Equal(t, http.StatusBadGateway, resp.StatusCode)
}

func TestHistoryHandler_RequestIDPresent(t *testing.T) {
	t.Parallel()

	app, mockUseCase := setupRouter(t)

	mockUseCase.EXPECT().History(gomock.Any()).Return(entity.TranslationHistory{}, nil)

	req := httptest.NewRequest(http.MethodGet, "/v1/translation/history", nil)
	resp, err := app.Test(req)

	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.NotEmpty(t, resp.Header.Get("X-Request-ID"))
}

// mockTranslation implements usecase.Translation for testing.
var _ usecase.Translation = (*MockTranslation)(nil)
