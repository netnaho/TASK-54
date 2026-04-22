package handler

import "github.com/gofiber/fiber/v2"

func Register(protected fiber.Router, h *OccupancyHandler) {
	protected.Get("/occupancy", h.Index)
}
