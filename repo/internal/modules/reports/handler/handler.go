package handler

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	authhandler "careops/clinic/internal/modules/auth/handler"
	"careops/clinic/internal/modules/reports/domain"
	reportsvc "careops/clinic/internal/modules/reports/service"
	"careops/clinic/internal/platform/idempotency"
	"careops/clinic/internal/shared/apperr"
	"careops/clinic/internal/shared/httpx"
	"github.com/gofiber/fiber/v2"
)

// ReportsHandler handles /reports routes.
type ReportsHandler struct {
	svc  *reportsvc.ReportService
	idem *idempotency.Service
}

// NewReportsHandler constructs the handler.
func NewReportsHandler(svc *reportsvc.ReportService, idem *idempotency.Service) *ReportsHandler {
	return &ReportsHandler{svc: svc, idem: idem}
}

// Index renders GET /reports — list of definitions and recent run history.
func (h *ReportsHandler) Index(c *fiber.Ctx) error {
	scheduled, err := h.svc.ListDefinitions()
	if err != nil {
		return err
	}
	runs, err := h.svc.ListRuns(20)
	if err != nil {
		return err
	}
	return httpx.RenderPage(c, "reports/index", httpx.PageData{
		Title: "Reports",
		User:  authhandler.CurrentUser(c),
		Flash: c.Query("flash"),
		Data: fiber.Map{
			"Scheduled":    scheduled,
			"Runs":         runs,
			"ValidTypes":   domain.ValidReportTypes,
			"ValidFormats": domain.ValidOutputFormats,
		},
	})
}

// NewForm renders GET /reports/new — report definition create form.
func (h *ReportsHandler) NewForm(c *fiber.Ctx) error {
	return httpx.RenderPage(c, "reports/new", httpx.PageData{
		Title: "New Report Definition",
		User:  authhandler.CurrentUser(c),
		Data: fiber.Map{
			"ValidTypes":   domain.ValidReportTypes,
			"ValidFormats": domain.ValidOutputFormats,
		},
	})
}

// Create handles POST /reports — persists a new report definition.
func (h *ReportsHandler) Create(c *fiber.Ctx) error {
	cu := authhandler.CurrentUser(c)

	idempKey := idempotency.ExtractKey(c)
	if idempKey != "" {
		if entry, err := h.idem.Check(idempKey); err == nil && entry != nil {
			return c.Redirect("/reports?flash=Report+already+created.")
		}
	}

	cmd := domain.CreateReportCmd{
		Name:         strings.TrimSpace(c.FormValue("name")),
		ReportType:   c.FormValue("report_type"),
		ScheduleCron: strings.TrimSpace(c.FormValue("schedule_cron")),
		OutputFormat: c.FormValue("output_format"),
		Parameters:   strings.TrimSpace(c.FormValue("parameters")),
	}

	sr, err := h.svc.CreateDefinition(cmd, cu.ID, c.IP(), c.Get("X-Request-ID"))
	if err != nil {
		return httpx.RenderPage(c, "reports/new", httpx.PageData{
			Title: "New Report Definition",
			User:  cu,
			Flash: err.Error(),
			Data: fiber.Map{
				"ValidTypes":   domain.ValidReportTypes,
				"ValidFormats": domain.ValidOutputFormats,
				"Cmd":          cmd,
			},
		})
	}

	if idempKey != "" {
		_ = h.idem.Store(idempKey, fiber.StatusFound, sr.ID)
	}
	return c.Redirect("/reports?flash=Report+definition+created.")
}

// RunNow handles POST /reports/:id/run — executes a report immediately.
func (h *ReportsHandler) RunNow(c *fiber.Ctx) error {
	cu := authhandler.CurrentUser(c)
	id := c.Params("id")

	idempKey := idempotency.ExtractKey(c)
	if idempKey != "" {
		if entry, err := h.idem.Check(idempKey); err == nil && entry != nil {
			return c.Redirect("/reports?flash=Report+run+already+submitted.")
		}
	}

	run, err := h.svc.RunReport(id, cu.ID, c.IP(), c.Get("X-Request-ID"))
	if err != nil {
		return c.Redirect("/reports?flash=" + url.QueryEscape("Run failed: "+err.Error()))
	}

	if idempKey != "" {
		_ = h.idem.Store(idempKey, fiber.StatusFound, run.ID)
	}
	return c.Redirect("/reports?flash=Report+run+completed+successfully.")
}

// Toggle handles POST /reports/:id/toggle — flips the is_active flag.
func (h *ReportsHandler) Toggle(c *fiber.Ctx) error {
	cu := authhandler.CurrentUser(c)
	id := c.Params("id")

	idempKey := idempotency.ExtractKey(c)
	if idempKey != "" {
		if entry, err := h.idem.Check(idempKey); err == nil && entry != nil {
			return c.Redirect("/reports?flash=Report+definition+already+updated.")
		}
	}

	if err := h.svc.ToggleActive(id, cu.ID, c.IP(), c.Get("X-Request-ID")); err != nil {
		return c.Redirect("/reports?flash=" + url.QueryEscape(err.Error()))
	}

	if idempKey != "" {
		_ = h.idem.Store(idempKey, fiber.StatusFound, id)
	}
	return c.Redirect("/reports?flash=Report+definition+updated.")
}

// DownloadOutput serves GET /reports/runs/download?run=<id> — downloads the
// output file for a completed run. Path traversal is blocked by using only
// the base file name from the stored output_path.
func (h *ReportsHandler) DownloadOutput(c *fiber.Ctx) error {
	runID := c.Query("run")
	if runID == "" {
		return fiber.ErrBadRequest
	}
	run, err := h.svc.FindRun(runID)
	if err != nil {
		return fiber.ErrInternalServerError
	}
	if run == nil || run.OutputPath == "" {
		return fiber.ErrNotFound
	}

	name := filepath.Base(run.OutputPath)
	if name == "" || name == "." {
		return fiber.ErrBadRequest
	}
	if _, err := os.Stat(run.OutputPath); err != nil {
		return fiber.ErrNotFound
	}

	contentType := "text/csv"
	if strings.HasSuffix(name, ".xlsx") {
		contentType = "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
	}
	c.Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, name))
	c.Set("Content-Type", contentType)
	return c.SendFile(run.OutputPath)
}

// EditForm renders GET /reports/:id/edit — pre-populated update form.
func (h *ReportsHandler) EditForm(c *fiber.Ctx) error {
	id := c.Params("id")
	sr, err := h.svc.FindDefinition(id)
	if err != nil || sr == nil {
		return fiber.ErrNotFound
	}
	return httpx.RenderPage(c, "reports/edit", httpx.PageData{
		Title: "Edit Report Definition",
		User:  authhandler.CurrentUser(c),
		Data: fiber.Map{
			"Report":       sr,
			"ValidTypes":   domain.ValidReportTypes,
			"ValidFormats": domain.ValidOutputFormats,
		},
	})
}

// Update handles POST /reports/:id/update — applies definition changes (OCC).
func (h *ReportsHandler) Update(c *fiber.Ctx) error {
	cu := authhandler.CurrentUser(c)
	id := c.Params("id")

	idempKey := idempotency.ExtractKey(c)
	if idempKey != "" {
		if entry, err := h.idem.Check(idempKey); err == nil && entry != nil {
			return c.Redirect("/reports?flash=Report+definition+already+updated.")
		}
	}

	rowVersion, _ := strconv.Atoi(c.FormValue("row_version"))
	cmd := domain.CreateReportCmd{
		Name:         strings.TrimSpace(c.FormValue("name")),
		ReportType:   c.FormValue("report_type"),
		ScheduleCron: strings.TrimSpace(c.FormValue("schedule_cron")),
		OutputFormat: c.FormValue("output_format"),
		Parameters:   strings.TrimSpace(c.FormValue("parameters")),
	}

	if err := h.svc.UpdateDefinition(id, cmd, cu.ID, c.IP(), c.Get("X-Request-ID"), rowVersion); err != nil {
		if errors.Is(err, apperr.ErrConflict) {
			return fiber.NewError(fiber.StatusConflict, "Record was modified by another request. Reload and try again.")
		}
		sr, _ := h.svc.FindDefinition(id)
		return httpx.RenderPage(c, "reports/edit", httpx.PageData{
			Title: "Edit Report Definition",
			User:  cu,
			Flash: err.Error(),
			Data: fiber.Map{
				"Report":       sr,
				"ValidTypes":   domain.ValidReportTypes,
				"ValidFormats": domain.ValidOutputFormats,
			},
		})
	}
	if idempKey != "" {
		_ = h.idem.Store(idempKey, fiber.StatusFound, id)
	}
	return c.Redirect("/reports?flash=Report+definition+updated.")
}
