package handler

import "github.com/gofiber/fiber/v2"

func Register(protected fiber.Router, h *AuditHandler) {
	protected.Get("/audit", h.Index)
}
