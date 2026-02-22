package middleware

import (
	"strconv"
	"strings"
	"time"

	"github.com/evrone/go-clean-template/pkg/logger"
	"github.com/gofiber/fiber/v2"
)

func buildRequestMessage(ctx *fiber.Ctx, latency time.Duration) string {
	var result strings.Builder

	// Request ID.
	if requestID, ok := ctx.Locals("request_id").(string); ok {
		result.WriteString("[")
		result.WriteString(requestID)
		result.WriteString("] ")
	}

	result.WriteString(ctx.IP())
	result.WriteString(" - ")
	result.WriteString(ctx.Method())
	result.WriteString(" ")
	result.WriteString(ctx.OriginalURL())
	result.WriteString(" - ")
	result.WriteString(strconv.Itoa(ctx.Response().StatusCode()))
	result.WriteString(" ")
	result.WriteString(strconv.Itoa(len(ctx.Response().Body())))
	result.WriteString(" - ")
	result.WriteString(latency.String())

	return result.String()
}

// Logger returns a middleware that logs request information including request ID and latency.
func Logger(l logger.Interface) func(c *fiber.Ctx) error {
	return func(ctx *fiber.Ctx) error {
		start := time.Now()

		err := ctx.Next()

		latency := time.Since(start)
		l.Info(buildRequestMessage(ctx, latency))

		return err
	}
}
