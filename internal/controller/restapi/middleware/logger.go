package middleware

import (
	"strconv"
	"strings"
	"time"

	"github.com/evrone/go-clean-template/pkg/logger"
	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/requestid"
)

func buildRequestMessage(ctx fiber.Ctx, latency time.Duration) string {
	var result strings.Builder

	if id := requestid.FromContext(ctx); id != "" {
		result.WriteString("[")
		result.WriteString(id)
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

func Logger(l logger.Interface) func(c fiber.Ctx) error {
	return func(ctx fiber.Ctx) error {
		start := time.Now()
		err := ctx.Next()

		l.Info("%s", buildRequestMessage(ctx, time.Since(start)))

		return err
	}
}
