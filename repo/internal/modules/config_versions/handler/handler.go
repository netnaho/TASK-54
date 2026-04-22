package handler

import (
	"errors"
	"strconv"
	"strings"

	authhandler "careops/clinic/internal/modules/auth/handler"
	"careops/clinic/internal/modules/config_versions/service"
	"careops/clinic/internal/platform/idempotency"
	"careops/clinic/internal/shared/apperr"
	"careops/clinic/internal/shared/httpx"
	"github.com/gofiber/fiber/v2"
)

// configFlash cookie name used for one-time post-redirect messages.
const configFlash = "careops_flash"

// ConfigVersionsHandler handles /config-versions routes.
type ConfigVersionsHandler struct {
	svc  *service.ConfigService
	idem *idempotency.Service
}

// NewConfigVersionsHandler constructs the handler.
func NewConfigVersionsHandler(svc *service.ConfigService, idem *idempotency.Service) *ConfigVersionsHandler {
	return &ConfigVersionsHandler{svc: svc, idem: idem}
}

// Index renders GET /config-versions.
func (h *ConfigVersionsHandler) Index(c *fiber.Ctx) error {
	configs, err := h.svc.List()
	if err != nil {
		return err
	}
	snaps, err := h.svc.ListSnapshots()
	if err != nil {
		return err
	}
	flash := c.Cookies(configFlash)
	if flash != "" {
		c.ClearCookie(configFlash)
	}
	return httpx.RenderPage(c, "config_versions/index", httpx.PageData{
		Title: "Configuration Versions",
		User:  authhandler.CurrentUser(c),
		Flash: flash,
		Data: fiber.Map{
			"Configs":   configs,
			"Snapshots": snaps,
		},
	})
}

// EditForm renders GET /config-versions/:id/edit.
func (h *ConfigVersionsHandler) EditForm(c *fiber.Ctx) error {
	cv, err := h.svc.FindByID(c.Params("id"))
	if err != nil {
		return fiber.ErrNotFound
	}
	return httpx.RenderPage(c, "config_versions/edit", httpx.PageData{
		Title: "Edit Config — " + cv.Namespace + "/" + cv.ConfigKey,
		User:  authhandler.CurrentUser(c),
		Data:  fiber.Map{"Config": cv},
	})
}

// Update handles POST /config-versions/:id.
func (h *ConfigVersionsHandler) Update(c *fiber.Ctx) error {
	cu := authhandler.CurrentUser(c)
	idemKey := idempotency.ExtractKey(c)

	if idemKey != "" {
		if entry, err := h.idem.Check(idemKey); err == nil && entry != nil {
			return httpx.RedirectWithFlash(c, "/config-versions", "Configuration updated successfully.")
		}
	}

	newValue := strings.TrimSpace(c.FormValue("config_value"))
	rowVersionStr := c.FormValue("row_version")
	rowVersion, err := strconv.Atoi(rowVersionStr)
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid row_version")
	}
	if err := h.svc.Update(c.Params("id"), newValue, cu.ID, c.IP(), c.Get("X-Request-ID"), rowVersion); err != nil {
		if errors.Is(err, apperr.ErrConflict) {
			return fiber.NewError(fiber.StatusConflict, "Record was modified by another request. Reload and try again.")
		}
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}
	if idemKey != "" {
		_ = h.idem.Store(idemKey, fiber.StatusFound, c.Params("id"))
	}
	return httpx.RedirectWithFlash(c, "/config-versions", "Configuration updated successfully.")
}

// SnapshotCreate handles POST /config-versions/snapshot.
func (h *ConfigVersionsHandler) SnapshotCreate(c *fiber.Ctx) error {
	cu := authhandler.CurrentUser(c)
	idemKey := idempotency.ExtractKey(c)

	if idemKey != "" {
		if entry, err := h.idem.Check(idemKey); err == nil && entry != nil {
			return httpx.RedirectWithFlash(c, "/config-versions", "Snapshot captured successfully.")
		}
	}

	snapPath, err := h.svc.TakeSnapshot(cu.ID, c.IP(), c.Get("X-Request-ID"))
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, err.Error())
	}
	if idemKey != "" {
		_ = h.idem.Store(idemKey, fiber.StatusFound, snapPath)
	}
	return httpx.RedirectWithFlash(c, "/config-versions", "Snapshot captured successfully.")
}

// Rollback handles POST /config-versions/rollback.
func (h *ConfigVersionsHandler) Rollback(c *fiber.Ctx) error {
	cu := authhandler.CurrentUser(c)
	idemKey := idempotency.ExtractKey(c)
	fileName := strings.TrimSpace(c.FormValue("snapshot_file"))

	if idemKey != "" {
		if entry, err := h.idem.Check(idemKey); err == nil && entry != nil {
			return httpx.RedirectWithFlash(c, "/config-versions", "Configuration rolled back successfully.")
		}
	}

	if fileName == "" {
		return fiber.NewError(fiber.StatusBadRequest, "snapshot_file is required")
	}
	if err := h.svc.Rollback(fileName, cu.ID, c.IP(), c.Get("X-Request-ID")); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}
	if idemKey != "" {
		_ = h.idem.Store(idemKey, fiber.StatusFound, fileName)
	}
	return httpx.RedirectWithFlash(c, "/config-versions", "Configuration rolled back successfully.")
}
