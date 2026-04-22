package handler

import (
	"fmt"

	authhandler "careops/clinic/internal/modules/auth/handler"
	exrepo "careops/clinic/internal/modules/exercise_library/repository"
	"careops/clinic/internal/shared/httpx"
	"github.com/gofiber/fiber/v2"
)

type ExerciseLibraryHandler struct {
	repo *exrepo.ExerciseRepo
}

func NewExerciseLibraryHandler(repo *exrepo.ExerciseRepo) *ExerciseLibraryHandler {
	return &ExerciseLibraryHandler{repo: repo}
}

// Index lists exercises with optional filtering.
// Accepts query params: category, difficulty, tag, body_part, q (text search), favorites=1.
// When HX-Request is set, returns only the exercise grid partial.
func (h *ExerciseLibraryHandler) Index(c *fiber.Ctx) error {
	cu := authhandler.CurrentUser(c)

	f := exrepo.ExerciseFilter{
		Category:      c.Query("category"),
		Difficulty:    c.Query("difficulty"),
		TagID:         c.Query("tag"),
		BodyPart:      c.Query("body_part"),
		Query:         c.Query("q"),
		FavoritesOnly: c.Query("favorites") == "1",
	}

	exercises, err := h.repo.List(f, cu.ID)
	if err != nil {
		return fmt.Errorf("exercise_library.Index: %w", err)
	}

	tags, err := h.repo.AllTags()
	if err != nil {
		return fmt.Errorf("exercise_library.Index tags: %w", err)
	}

	categories, err := h.repo.AllCategories()
	if err != nil {
		return fmt.Errorf("exercise_library.Index categories: %w", err)
	}

	bodyParts, err := h.repo.AllBodyParts()
	if err != nil {
		return fmt.Errorf("exercise_library.Index body_parts: %w", err)
	}

	data := fiber.Map{
		"Exercises":  exercises,
		"Tags":       tags,
		"Categories": categories,
		"BodyParts":  bodyParts,
		"Filter":     f,
		"HasFilters": f.Category != "" || f.Difficulty != "" || f.TagID != "" || f.BodyPart != "" || f.Query != "" || f.FavoritesOnly,
	}

	if c.Get("HX-Request") == "true" {
		return httpx.RenderPartial(c, "exercise_library/grid_partial", data)
	}
	return httpx.RenderPage(c, "exercise_library/index", httpx.PageData{
		Title: "Exercise Library",
		User:  cu,
		Data:  data,
	})
}

// Detail renders the full exercise detail page.
func (h *ExerciseLibraryHandler) Detail(c *fiber.Ctx) error {
	cu := authhandler.CurrentUser(c)
	id := c.Params("id")

	ex, err := h.repo.FindByID(id, cu.ID)
	if err != nil {
		return fmt.Errorf("exercise_library.Detail: %w", err)
	}
	if ex == nil {
		return fiber.NewError(fiber.StatusNotFound, "Exercise not found.")
	}

	return httpx.RenderPage(c, "exercise_library/detail", httpx.PageData{
		Title: ex.Name,
		User:  cu,
		Data:  fiber.Map{"Exercise": ex},
	})
}

// ToggleFavorite adds or removes the exercise from the user's favorites.
// Returns an HTMX-ready fragment with the updated favorite button.
func (h *ExerciseLibraryHandler) ToggleFavorite(c *fiber.Ctx) error {
	cu := authhandler.CurrentUser(c)
	id := c.Params("id")

	isFav, err := h.repo.ToggleFavorite(cu.ID, id)
	if err != nil {
		return fmt.Errorf("exercise_library.ToggleFavorite: %w", err)
	}

	// Return an HTMX fragment — just the updated button
	return httpx.RenderPartial(c, "exercise_library/favorite_btn", fiber.Map{
		"ExerciseID": id,
		"IsFavorite": isFav,
	})
}
