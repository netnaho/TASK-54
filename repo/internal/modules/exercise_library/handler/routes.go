package handler

import "github.com/gofiber/fiber/v2"

func Register(protected fiber.Router, h *ExerciseLibraryHandler) {
	protected.Get("/exercise-library", h.Index)
}
