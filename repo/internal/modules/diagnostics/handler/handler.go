package handler

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	authhandler "careops/clinic/internal/modules/auth/handler"
	diagsvc "careops/clinic/internal/modules/diagnostics/service"
	"careops/clinic/internal/platform/idempotency"
	"careops/clinic/internal/shared/httpx"
	"github.com/gofiber/fiber/v2"
)

// DiagnosticsHandler handles /diagnostics routes.
type DiagnosticsHandler struct {
	svc      *diagsvc.DiagnosticsService
	idem     *idempotency.Service
	diagPath string
}

// NewDiagnosticsHandler constructs the handler.
func NewDiagnosticsHandler(svc *diagsvc.DiagnosticsService, idem *idempotency.Service, diagPath string) *DiagnosticsHandler {
	return &DiagnosticsHandler{svc: svc, idem: idem, diagPath: diagPath}
}

// Index renders GET /diagnostics.
func (h *DiagnosticsHandler) Index(c *fiber.Ctx) error {
	exports, err := h.svc.ListExports(20)
	if err != nil {
		return err
	}
	stats, err := h.svc.DBStats()
	if err != nil {
		return err
	}
	flash := c.Query("flash")
	return httpx.RenderPage(c, "diagnostics/index", httpx.PageData{
		Title: "Diagnostics",
		User:  authhandler.CurrentUser(c),
		Flash: flash,
		Data: fiber.Map{
			"Exports": exports,
			"DBStats": stats,
		},
	})
}

// ExportCreate handles POST /diagnostics/exports.
func (h *DiagnosticsHandler) ExportCreate(c *fiber.Ctx) error {
	cu := authhandler.CurrentUser(c)
	idemKey := idempotency.ExtractKey(c)

	if idemKey != "" {
		if entry, err := h.idem.Check(idemKey); err == nil && entry != nil {
			return c.Redirect("/diagnostics?flash=Diagnostic+package+already+created.")
		}
	}

	pkg, err := h.svc.ExportPackage(cu.ID, c.IP(), c.Get("X-Request-ID"))
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}
	if idemKey != "" {
		_ = h.idem.Store(idemKey, fiber.StatusFound, pkg)
	}
	return c.Redirect("/diagnostics?flash=Diagnostic+package+created+successfully.")
}

// ExportDownload serves GET /diagnostics/exports/download?file=<name>.
// Path traversal is blocked: only base filenames under diagPath are served.
func (h *DiagnosticsHandler) ExportDownload(c *fiber.Ctx) error {
	name := filepath.Base(c.Query("file"))
	if name == "" || name == "." {
		return fiber.ErrBadRequest
	}
	fullPath := filepath.Join(h.diagPath, name)
	rel, err := filepath.Rel(h.diagPath, fullPath)
	if err != nil || strings.HasPrefix(rel, "..") {
		return fiber.ErrBadRequest
	}
	if _, err := os.Stat(fullPath); err != nil {
		return fiber.ErrNotFound
	}
	c.Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, name))
	return c.SendFile(fullPath)
}
