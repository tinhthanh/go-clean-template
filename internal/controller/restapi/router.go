// Package v1 implements routing paths. Each services in own file.
package restapi

import (
	"net/http"

	"github.com/ansrivas/fiberprometheus/v2"
	"github.com/evrone/go-clean-template/config"
	_ "github.com/evrone/go-clean-template/docs" // Swagger docs.
	"github.com/evrone/go-clean-template/internal/controller/restapi/middleware"
	v1 "github.com/evrone/go-clean-template/internal/controller/restapi/v1"
	"github.com/evrone/go-clean-template/internal/usecase"
	"github.com/evrone/go-clean-template/pkg/logger"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/swagger"
)

// NewRouter -.
// Swagger spec:
// @title       Go Clean Template API
// @description Using a translation service as an example
// @version     1.0
// @host        localhost:8080
// @BasePath    /v1
func NewRouter(app *fiber.App, cfg *config.Config, t usecase.Translation, l logger.Interface) {
	// Middleware — order matters: RequestID → Security → CORS → RateLimit → Logger → Recovery.
	app.Use(middleware.RequestID())
	app.Use(middleware.Security())
	app.Use(middleware.CORS(middleware.CORSConfig{
		AllowOrigins: cfg.CORS.AllowOrigins,
		AllowMethods: "GET,POST,PUT,DELETE,PATCH,OPTIONS",
		AllowHeaders: "Origin,Content-Type,Accept,Authorization,X-Request-ID",
	}))
	app.Use(middleware.RateLimit(middleware.RateLimitConfig{
		Max:        cfg.RateLimit.Max,
		Expiration: cfg.RateLimit.Expiration,
	}))
	app.Use(middleware.Logger(l))
	app.Use(middleware.Recovery(l))

	// OpenTelemetry tracing (conditional).
	if cfg.Tracer.Enabled {
		app.Use(middleware.Tracing(cfg.Tracer.ServiceName, l))
	}

	// Prometheus metrics.
	if cfg.Metrics.Enabled {
		prometheus := fiberprometheus.New(cfg.App.Name)
		prometheus.RegisterAt(app, "/metrics")
		app.Use(prometheus.Middleware)
	}

	// Swagger.
	if cfg.Swagger.Enabled {
		app.Get("/swagger/*", swagger.HandlerDefault)
	}

	// K8s probe.
	app.Get("/healthz", func(ctx *fiber.Ctx) error { return ctx.SendStatus(http.StatusOK) })

	// Routers.
	apiV1Group := app.Group("/v1")
	{
		v1.NewTranslationRoutes(apiV1Group, t, l)
	}
}
