package middleware

import (
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/limiter"
)

// RateLimitConfig holds rate limiting configuration.
type RateLimitConfig struct {
	Max        int
	Expiration time.Duration
}

// DefaultRateLimitConfig returns sensible default rate limiting configuration.
func DefaultRateLimitConfig() RateLimitConfig {
	return RateLimitConfig{
		Max:        100,
		Expiration: 1 * time.Minute,
	}
}

// RateLimit returns a rate limiting middleware per IP.
func RateLimit(cfg RateLimitConfig) func(c *fiber.Ctx) error {
	return limiter.New(limiter.Config{
		Max:        cfg.Max,
		Expiration: cfg.Expiration,
		KeyGenerator: func(c *fiber.Ctx) string {
			return c.IP()
		},
		LimitReached: func(c *fiber.Ctx) error {
			return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{
				"code":    "RATE_LIMIT_EXCEEDED",
				"message": "too many requests, please try again later",
			})
		},
	})
}
