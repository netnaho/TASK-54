package handler

import "github.com/gofiber/fiber/v2"

// Register wires diagnostics routes with the provided middleware chain.
func Register(r fiber.Router, h *DiagnosticsHandler, middleware ...fiber.Handler) {
	mw := middleware
	r.Get("/diagnostics", append(mw, h.Index)...)
	r.Post("/diagnostics/exports", append(mw, h.ExportCreate)...)
	r.Get("/diagnostics/exports/download", append(mw, h.ExportDownload)...)
}
