package handler

import (
	"fmt"

	authhandler "careops/clinic/internal/modules/auth/handler"
	"careops/clinic/internal/modules/beds/repository"
	"careops/clinic/internal/shared/httpx"
	"github.com/gofiber/fiber/v2"
)

type BedHandler struct {
	repo *repository.BedRepo
}

func NewBedHandler(repo *repository.BedRepo) *BedHandler {
	return &BedHandler{repo: repo}
}

// Index lists all active beds with occupancy state.
// Accepts: ward (filter), status (occupied|available|all).
// When HX-Request is set, returns only the beds table partial.
func (h *BedHandler) Index(c *fiber.Ctx) error {
	cu := authhandler.CurrentUser(c)
	wardFilter := c.Query("ward")
	statusFilter := c.Query("status") // "occupied" | "available" | ""

	var bedList interface{}
	var err error
	if wardFilter != "" {
		bedList, err = h.repo.ListByWard(wardFilter)
	} else {
		bedList, err = h.repo.ListWithOccupancy()
	}
	if err != nil {
		return fmt.Errorf("bed_handler.Index: %w", err)
	}

	total, occupied, err := h.repo.Counts()
	if err != nil {
		return fmt.Errorf("bed_handler.Index counts: %w", err)
	}
	available := total - occupied

	wards, err := h.repo.DistinctWards()
	if err != nil {
		return fmt.Errorf("bed_handler.Index wards: %w", err)
	}

	data := fiber.Map{
		"Beds":         bedList,
		"Total":        total,
		"Occupied":     occupied,
		"Available":    available,
		"Wards":        wards,
		"WardFilter":   wardFilter,
		"StatusFilter": statusFilter,
	}

	if c.Get("HX-Request") == "true" {
		return httpx.RenderPartial(c, "beds/table_partial", data)
	}
	return httpx.RenderPage(c, "beds/index", httpx.PageData{
		Title: "Bed Management",
		User:  cu,
		Data:  data,
	})
}

// Detail renders the detail view for a single bed.
func (h *BedHandler) Detail(c *fiber.Ctx) error {
	cu := authhandler.CurrentUser(c)
	id := c.Params("id")

	bed, err := h.repo.FindByID(id)
	if err != nil {
		return fmt.Errorf("bed_handler.Detail: %w", err)
	}
	if bed == nil {
		return fiber.NewError(fiber.StatusNotFound, "Bed not found.")
	}

	return httpx.RenderPage(c, "beds/detail", httpx.PageData{
		Title: "Bed " + bed.Ward + " " + bed.RoomNumber + "-" + bed.BedLabel,
		User:  cu,
		Data:  fiber.Map{"Bed": bed},
	})
}
