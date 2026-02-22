package middleware

import (
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
)

// CORSConfig holds CORS configuration.
type CORSConfig struct {
	AllowOrigins string
	AllowMethods string
	AllowHeaders string
}

// DefaultCORSConfig returns sensible default CORS configuration.
func DefaultCORSConfig() CORSConfig {
	return CORSConfig{
		AllowOrigins: "*",
		AllowMethods: "GET,POST,PUT,DELETE,PATCH,OPTIONS",
		AllowHeaders: "Origin,Content-Type,Accept,Authorization,X-Request-ID",
	}
}

// CORS returns a CORS middleware with the given configuration.
func CORS(cfg CORSConfig) func(c *fiber.Ctx) error {
	return cors.New(cors.Config{
		AllowOrigins:  cfg.AllowOrigins,
		AllowMethods:  cfg.AllowMethods,
		AllowHeaders:  cfg.AllowHeaders,
		ExposeHeaders: "X-Request-ID",
	})
}
