package handler

import "github.com/gofiber/fiber/v2"

// Register wires config-versions routes with the provided middleware chain.
// NOTE: Specific static-suffix POST routes (/snapshot, /rollback) must be
// registered BEFORE the wildcard POST /config-versions/:id, otherwise Fiber
// routes snapshot/rollback through the Update handler.
func Register(r fiber.Router, h *ConfigVersionsHandler, middleware ...fiber.Handler) {
	mw := middleware
	r.Get("/config-versions", append(mw, h.Index)...)
	r.Post("/config-versions/snapshot", append(mw, h.SnapshotCreate)...)
	r.Post("/config-versions/rollback", append(mw, h.Rollback)...)
	r.Get("/config-versions/:id/edit", append(mw, h.EditForm)...)
	r.Post("/config-versions/:id", append(mw, h.Update)...)
}
