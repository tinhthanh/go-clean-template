package middleware

import (
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/helmet"
)

// Security returns a middleware that sets security headers using Fiber's helmet middleware.
// Headers include: X-Content-Type-Options, X-Frame-Options, X-XSS-Protection, etc.
func Security() func(c *fiber.Ctx) error {
	return helmet.New()
}
