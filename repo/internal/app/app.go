package app

import (
	"database/sql"
	"log/slog"
	"time"

	"careops/clinic/internal/config"
	tmplengine "careops/clinic/internal/platform/template"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/compress"
)

// New builds and returns a configured Fiber application without starting it.
func New(cfg *config.Config, db *sql.DB, log *slog.Logger) *fiber.App {
	tz, err := time.LoadLocation(cfg.FacilityTimezone)
	if err != nil {
		log.Warn("invalid FACILITY_TIMEZONE, falling back to UTC", "tz", cfg.FacilityTimezone, "err", err)
		tz = time.UTC
	}
	engine := tmplengine.NewEngine(cfg.TemplatesPath, tz)

	app := fiber.New(fiber.Config{
		AppName:               "CareOps",
		Views:                 engine,
		ErrorHandler:          buildErrorHandler(log),
		DisableStartupMessage: true,
	})

	app.Use(compress.New())

	registerMiddleware(app, cfg, log)
	registerRoutes(app, cfg, db, log)

	return app
}

// buildErrorHandler returns a Fiber error handler that:
//   - Detects HTMX requests (HX-Request: true) and returns fragment-friendly responses
//   - Handles 401 → redirect to login (or 401 JSON for HTMX)
//   - Handles 403 → 403 page (or 403 JSON + HX-Redirect for HTMX)
//   - Handles 404 → 404 page
//   - Handles 409 → 409 page (or 409 JSON for HTMX)
//   - Handles 500 → 500 page
//
// HTMX partial responses use HX-Redirect for auth failures so the browser
// redirects the full page, not just the swapped fragment.
func buildErrorHandler(log *slog.Logger) fiber.ErrorHandler {
	return func(c *fiber.Ctx, err error) error {
		code := fiber.StatusInternalServerError
		msg := "An unexpected error occurred."

		if e, ok := err.(*fiber.Error); ok { //nolint:errorlint
			code = e.Code
			msg = e.Message
		}

		log.Error("request error",
			"status", code,
			"err", err.Error(),
			"path", c.Path(),
			"method", c.Method(),
		)

		isHTMX := c.Get("HX-Request") == "true"

		// HTMX requests get lightweight JSON responses with appropriate Fiber headers.
		// Full-page requests get rendered HTML error pages.
		switch code {
		case fiber.StatusUnauthorized:
			if isHTMX {
				c.Set("HX-Redirect", "/login")
				return c.Status(code).JSON(fiber.Map{"error": msg})
			}
			return c.Redirect("/login")

		case fiber.StatusForbidden:
			if isHTMX {
				c.Set("HX-Redirect", "/dashboard")
				return c.Status(code).JSON(fiber.Map{"error": msg})
			}
			return c.Status(code).Render("errors/403",
				fiber.Map{"Title": "Access Denied", "Message": msg},
				"layouts/base",
			)

		case fiber.StatusNotFound:
			if isHTMX {
				return c.Status(code).JSON(fiber.Map{"error": "Page not found."})
			}
			return c.Status(code).Render("errors/404",
				fiber.Map{"Title": "Page Not Found"},
				"layouts/base",
			)

		case fiber.StatusConflict:
			if isHTMX {
				return c.Status(code).JSON(fiber.Map{
					"error": "The record was modified by another request. Please reload and try again.",
				})
			}
			return c.Status(code).Render("errors/409",
				fiber.Map{"Title": "Record Changed", "Message": msg},
				"layouts/base",
			)

		default:
			if isHTMX {
				return c.Status(code).JSON(fiber.Map{"error": "An internal error occurred."})
			}
			return c.Status(code).Render("errors/500",
				fiber.Map{"Title": "Internal Server Error"},
				"layouts/base",
			)
		}
	}
}
