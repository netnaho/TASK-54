package handler

import (
	authhandler "careops/clinic/internal/modules/auth/handler"
	"careops/clinic/internal/modules/audit/domain"
	"careops/clinic/internal/modules/audit/service"
	"careops/clinic/internal/shared/httpx"
	"github.com/gofiber/fiber/v2"
)

type AuditHandler struct {
	svc *service.AuditService
}

func NewAuditHandler(svc *service.AuditService) *AuditHandler {
	return &AuditHandler{svc: svc}
}

// Index renders the audit log page with optional query filters.
// Supported query params: entity_type, entity_id, user_id, resident_id, date_from, date_to.
func (h *AuditHandler) Index(c *fiber.Ctx) error {
	f := domain.ListFilter{
		EntityType: truncate(c.Query("entity_type"), 64),
		EntityID:   truncate(c.Query("entity_id"), 64),
		UserID:     truncate(c.Query("user_id"), 64),
		ResidentID: truncate(c.Query("resident_id"), 64),
		DateFrom:   truncate(c.Query("date_from"), 10),
		DateTo:     truncate(c.Query("date_to"), 10),
		Limit:      200,
	}

	logs, err := h.svc.ListFiltered(f)
	if err != nil {
		return err
	}
	count, err := h.svc.Count()
	if err != nil {
		return err
	}

	return httpx.RenderPage(c, "audit/index", httpx.PageData{
		Title: "Audit Log",
		User:  authhandler.CurrentUser(c),
		Data: fiber.Map{
			"Logs":       logs,
			"Count":      count,
			"Filter":     f,
			"HasFilters": f.EntityType != "" || f.EntityID != "" || f.UserID != "" || f.ResidentID != "" || f.DateFrom != "" || f.DateTo != "",
		},
	})
}

// truncate caps a string at maxLen characters to prevent unbounded query params.
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen]
}
