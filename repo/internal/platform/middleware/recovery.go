package middleware

import (
	"log/slog"
	"runtime/debug"

	"github.com/gofiber/fiber/v2"
)

// Recovery catches panics, logs the stack trace, and returns a 500 response
// instead of crashing the process.
func Recovery(log *slog.Logger) fiber.Handler {
	return func(c *fiber.Ctx) (err error) {
		defer func() {
			if r := recover(); r != nil {
				log.Error("panic recovered",
					"panic",      r,
					"stack",      string(debug.Stack()),
					"request_id", GetRequestID(c),
					"path",       c.Path(),
				)
				err = c.Status(fiber.StatusInternalServerError).
					Render("errors/500", fiber.Map{"Title": "Internal Server Error"}, "layouts/base")
				if err != nil {
					// If rendering fails, fall back to plain text.
					_ = c.Status(fiber.StatusInternalServerError).SendString("Internal server error")
				}
			}
		}()
		return c.Next()
	}
}
