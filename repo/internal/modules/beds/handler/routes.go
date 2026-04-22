package handler

import "github.com/gofiber/fiber/v2"

func Register(protected fiber.Router, h *BedHandler) {
	protected.Get("/beds", h.Index)
}
