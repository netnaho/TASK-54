package handler

import "github.com/gofiber/fiber/v2"

// Register wires all reports routes with the provided middleware chain.
// Static routes (/reports/new, /reports/runs/download) are registered before
// parameterised routes (:id) to prevent prefix shadowing.
func Register(r fiber.Router, h *ReportsHandler, middleware ...fiber.Handler) {
	mw := middleware
	r.Get("/reports", append(mw, h.Index)...)
	r.Get("/reports/new", append(mw, h.NewForm)...)
	r.Post("/reports", append(mw, h.Create)...)
	r.Get("/reports/runs/download", append(mw, h.DownloadOutput)...)
	r.Get("/reports/:id/edit", append(mw, h.EditForm)...)
	r.Post("/reports/:id/update", append(mw, h.Update)...)
	r.Post("/reports/:id/run", append(mw, h.RunNow)...)
	r.Post("/reports/:id/toggle", append(mw, h.Toggle)...)
}
