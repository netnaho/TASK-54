package handler

import "github.com/gofiber/fiber/v2"

func Register(protected fiber.Router, h *ServiceDeliveryHandler) {
	protected.Get("/service-delivery", h.Index)
}
