package middleware

import (
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

const RequestIDHeader = "X-Request-ID"

// RequestID generates a unique request ID for each request and stores it in context locals.
func RequestID() func(c *fiber.Ctx) error {
	return func(ctx *fiber.Ctx) error {
		// Use existing request ID from header if provided, otherwise generate new one.
		requestID := ctx.Get(RequestIDHeader)
		if requestID == "" {
			requestID = uuid.New().String()
		}

		ctx.Locals("request_id", requestID)
		ctx.Set(RequestIDHeader, requestID)

		return ctx.Next()
	}
}
