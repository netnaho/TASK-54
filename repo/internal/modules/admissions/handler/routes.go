package handler

import "github.com/gofiber/fiber/v2"

// Register wires the admissions routes with the supplied middleware chain.
// Using the variadic pattern (consistent with other modules) prevents
// middleware bleed when Fiber routes are registered on the same router.
func Register(r fiber.Router, h *AdmissionHandler, middleware ...fiber.Handler) {
	mw := middleware
	r.Get("/admissions", append(mw, h.Index)...)
	r.Get("/admissions/:id", append(mw, h.ResidentDetail)...)
}
