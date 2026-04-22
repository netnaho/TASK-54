package handler

import "github.com/gofiber/fiber/v2"

func Register(protected fiber.Router, h *JobsHandler) {
	protected.Get("/jobs", h.Index)
}
