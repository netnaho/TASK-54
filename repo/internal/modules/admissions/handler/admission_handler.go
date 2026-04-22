package handler

import (
	"fmt"

	authhandler "careops/clinic/internal/modules/auth/handler"
	"careops/clinic/internal/modules/admissions/repository"
	"careops/clinic/internal/shared/httpx"
	"github.com/gofiber/fiber/v2"
)

type AdmissionHandler struct {
	repo *repository.AdmissionRepo
}

func NewAdmissionHandler(repo *repository.AdmissionRepo) *AdmissionHandler {
	return &AdmissionHandler{repo: repo}
}

// Index lists residents with admission status.
// Accepts: care_level, status (admitted|all).
// When HX-Request is set, returns only the residents table partial.
func (h *AdmissionHandler) Index(c *fiber.Ctx) error {
	cu := authhandler.CurrentUser(c)
	careLevelFilter := c.Query("care_level")
	statusFilter := c.Query("status", "admitted")

	residents, err := h.repo.ListResidentsWithStatus()
	if err != nil {
		return fmt.Errorf("admission_handler.Index: %w", err)
	}

	// Apply in-memory filters (data set is small)
	if careLevelFilter != "" || statusFilter == "admitted" {
		filtered := residents[:0]
		for _, r := range residents {
			if careLevelFilter != "" && r.CareLevel != careLevelFilter {
				continue
			}
			if statusFilter == "admitted" && !r.IsCurrentlyAdmitted {
				continue
			}
			filtered = append(filtered, r)
		}
		residents = filtered
	}

	totalRes, activeAdm, err := h.repo.Counts()
	if err != nil {
		return fmt.Errorf("admission_handler.Index counts: %w", err)
	}

	data := fiber.Map{
		"Residents":        residents,
		"TotalResidents":   totalRes,
		"ActiveAdmissions": activeAdm,
		"CareLevel":        careLevelFilter,
		"StatusFilter":     statusFilter,
	}

	if c.Get("HX-Request") == "true" {
		return httpx.RenderPartial(c, "admissions/table_partial", data)
	}
	return httpx.RenderPage(c, "admissions/index", httpx.PageData{
		Title: "Admissions & Residents",
		User:  cu,
		Data:  data,
	})
}

// ResidentDetail renders a single resident's record.
func (h *AdmissionHandler) ResidentDetail(c *fiber.Ctx) error {
	cu := authhandler.CurrentUser(c)
	id := c.Params("id")

	resident, err := h.repo.FindResident(id)
	if err != nil {
		return fmt.Errorf("admission_handler.ResidentDetail: %w", err)
	}
	if resident == nil {
		return fiber.NewError(fiber.StatusNotFound, "Resident not found.")
	}

	return httpx.RenderPage(c, "admissions/detail", httpx.PageData{
		Title: resident.FullName(),
		User:  cu,
		Data:  fiber.Map{"Resident": resident},
	})
}
