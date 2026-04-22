package handler

import "github.com/gofiber/fiber/v2"

// Register wires all exam scheduler routes onto the router with the provided middleware chain.
// Middleware (RequireSession + RequireRole) must be supplied by the caller so that
// app.Group("", ...) is never used — empty-prefix groups in Fiber v2 bleed middleware globally.
func Register(r fiber.Router, h *ExamHandler, middleware ...fiber.Handler) {
	mw := middleware

	// Overview
	r.Get("/exams", append(mw, h.Index)...)
	r.Get("/exams/scheduler", append(mw, h.Scheduler)...)

	// Template management
	r.Get("/exams/templates/new", append(mw, h.TemplateNew)...)
	r.Post("/exams/templates", append(mw, h.TemplateCreate)...)
	r.Get("/exams/templates/:id", append(mw, h.TemplateDetail)...)
	r.Post("/exams/templates/:id", append(mw, h.TemplateUpdate)...)

	// Session lifecycle
	r.Get("/exams/sessions/new", append(mw, h.SessionNew)...)
	r.Post("/exams/sessions", append(mw, h.SessionCreate)...)
	r.Get("/exams/sessions/:id", append(mw, h.SessionDetail)...)
	r.Post("/exams/sessions/:id/block", append(mw, h.SessionBlockUpdate)...)
	r.Post("/exams/sessions/:id/publish", append(mw, h.SessionPublish)...)
	r.Post("/exams/sessions/:id/candidates/add", append(mw, h.SessionAddCandidate)...)
	r.Post("/exams/sessions/:id/candidates/remove", append(mw, h.SessionRemoveCandidate)...)

	// Conflict resolution
	r.Post("/exams/conflicts/:id/resolve", append(mw, h.ConflictResolve)...)
}
