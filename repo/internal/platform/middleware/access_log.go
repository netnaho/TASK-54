package middleware

import (
	"log/slog"
	"time"

	"github.com/gofiber/fiber/v2"
)

// AccessLog logs one structured line per request after the handler chain completes.
// It records method, path, status, latency, and request ID.
func AccessLog(log *slog.Logger) fiber.Handler {
	return func(c *fiber.Ctx) error {
		start := time.Now()
		err := c.Next()
		latency := time.Since(start)

		log.Info("request",
			"method",     c.Method(),
			"path",       c.Path(),
			"status",     c.Response().StatusCode(),
			"latency_ms", latency.Milliseconds(),
			"request_id", GetRequestID(c),
			"ip",         c.IP(),
		)
		return err
	}
}
