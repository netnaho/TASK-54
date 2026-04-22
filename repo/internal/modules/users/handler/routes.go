package handler

import "github.com/gofiber/fiber/v2"

// Register wires user management routes using the explicit per-route middleware pattern.
func Register(r fiber.Router, h *UserHandler, middleware ...fiber.Handler) {
	mw := middleware
	r.Get("/users", append(mw, h.Index)...)
	r.Get("/users/new", append(mw, h.NewForm)...)
	r.Post("/users", append(mw, h.Create)...)
	r.Get("/users/:id/reset-password", append(mw, h.ResetPasswordForm)...)
	r.Post("/users/:id/reset-password", append(mw, h.ResetPassword)...)
}
