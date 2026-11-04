package restapi_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/evrone/go-clean-template/config"
	"github.com/evrone/go-clean-template/internal/controller/restapi"
	"github.com/evrone/go-clean-template/pkg/jwt"
	"github.com/evrone/go-clean-template/pkg/logger"
	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const _origin = "https://app.example.com"

// newRouter is the REST router with no use cases: these tests only reach
// /healthz, which needs none.
func newRouter(t *testing.T, limit int) *fiber.App {
	t.Helper()

	cfg := &config.Config{}
	cfg.CORS.AllowOrigins = []string{_origin}
	cfg.Limit.Max = limit
	cfg.Limit.Expiration = time.Minute

	app := fiber.New()
	restapi.NewRouter(app, cfg, nil, nil, nil, jwt.New("test-secret", time.Hour), logger.New("error"))

	return app
}

// get answers the status and headers of GET /healthz, the body closed.
func get(t *testing.T, app *fiber.App, origin string) (int, http.Header) {
	t.Helper()

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/healthz", http.NoBody)
	if origin != "" {
		req.Header.Set("Origin", origin)
	}

	resp, err := app.Test(req)
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())

	return resp.StatusCode, resp.Header
}

func TestRouter_EveryAnswerCarriesARequestID(t *testing.T) {
	t.Parallel()

	status, header := get(t, newRouter(t, 10), "")

	assert.Equal(t, http.StatusOK, status)
	assert.NotEmpty(t, header.Get("X-Request-ID"))
}

func TestRouter_SecurityHeadersAreSet(t *testing.T) {
	t.Parallel()

	_, header := get(t, newRouter(t, 10), "")

	assert.Equal(t, "nosniff", header.Get("X-Content-Type-Options"))
	assert.NotEmpty(t, header.Get("X-Frame-Options"))
}

func TestRouter_CORSAllowsOnlyTheConfiguredOrigin(t *testing.T) {
	t.Parallel()

	app := newRouter(t, 10)

	_, allowed := get(t, app, _origin)
	_, other := get(t, app, "https://elsewhere.example.com")

	assert.Equal(t, _origin, allowed.Get("Access-Control-Allow-Origin"))
	assert.Empty(t, other.Get("Access-Control-Allow-Origin"))
}

func TestRouter_ACallerOverTheLimitIsTurnedAway(t *testing.T) {
	t.Parallel()

	app := newRouter(t, 2)

	for range 2 {
		status, _ := get(t, app, "")
		assert.Equal(t, http.StatusOK, status)
	}

	status, _ := get(t, app, "")
	assert.Equal(t, http.StatusTooManyRequests, status)
}
