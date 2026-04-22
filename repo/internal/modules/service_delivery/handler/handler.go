package handler

import (
	"fmt"
	"log/slog"
	"net/http"
	"strconv"

	admissionsrepo "careops/clinic/internal/modules/admissions/repository"
	auditsvc "careops/clinic/internal/modules/audit/service"
	audithelper "careops/clinic/internal/shared/audit"
	authhandler "careops/clinic/internal/modules/auth/handler"
	sddomain "careops/clinic/internal/modules/service_delivery/domain"
	sdrepo "careops/clinic/internal/modules/service_delivery/repository"
	"careops/clinic/internal/platform/idempotency"
	"careops/clinic/internal/shared/httpx"
	"careops/clinic/internal/shared/validation"
	"github.com/gofiber/fiber/v2"
)

type ServiceDeliveryHandler struct {
	orderRepo      *sdrepo.ServiceOrderRepo
	executionRepo  *sdrepo.ExecutionRecordRepo
	alertRepo      *sdrepo.AlertRepo
	checkpointRepo *sdrepo.CareCheckpointRepo
	admissionRepo  *admissionsrepo.AdmissionRepo
	idempotency    *idempotency.Service
	auditSvc       *auditsvc.AuditService
	log            *slog.Logger
}

func NewServiceDeliveryHandler(
	orderRepo *sdrepo.ServiceOrderRepo,
	executionRepo *sdrepo.ExecutionRecordRepo,
	alertRepo *sdrepo.AlertRepo,
	checkpointRepo *sdrepo.CareCheckpointRepo,
	admissionRepo *admissionsrepo.AdmissionRepo,
	idp *idempotency.Service,
	auditSvc *auditsvc.AuditService,
	log *slog.Logger,
) *ServiceDeliveryHandler {
	return &ServiceDeliveryHandler{
		orderRepo:      orderRepo,
		executionRepo:  executionRepo,
		alertRepo:      alertRepo,
		checkpointRepo: checkpointRepo,
		admissionRepo:  admissionRepo,
		idempotency:    idp,
		auditSvc:       auditSvc,
		log:            log,
	}
}

// Index renders the service delivery dashboard with metrics and live tables.
// Accepts query params: date_from, date_to, resident_id, status, service_type.
// When HX-Request is set, renders only the content partial for HTMX swaps.
func (h *ServiceDeliveryHandler) Index(c *fiber.Ctx) error {
	cu := authhandler.CurrentUser(c)

	dateFrom := c.Query("date_from")
	dateTo := c.Query("date_to")
	residentID := c.Query("resident_id")
	status := c.Query("status", "active")
	serviceType := c.Query("service_type")

	metrics, err := h.executionRepo.ComputeMetrics(dateFrom, dateTo)
	if err != nil {
		return fmt.Errorf("service_delivery.Index metrics: %w", err)
	}

	orders, err := h.orderRepo.List(sdrepo.OrderFilter{
		ResidentID:  residentID,
		Status:      status,
		ServiceType: serviceType,
		DateFrom:    dateFrom,
		DateTo:      dateTo,
	})
	if err != nil {
		return fmt.Errorf("service_delivery.Index orders: %w", err)
	}

	records, err := h.executionRepo.List(sdrepo.ExecutionFilter{
		DateFrom: dateFrom,
		DateTo:   dateTo,
	})
	if err != nil {
		return fmt.Errorf("service_delivery.Index records: %w", err)
	}

	alerts, err := h.alertRepo.ListUnresolved()
	if err != nil {
		return fmt.Errorf("service_delivery.Index alerts: %w", err)
	}

	checkpoints, err := h.checkpointRepo.List(residentID, dateFrom, dateTo)
	if err != nil {
		return fmt.Errorf("service_delivery.Index checkpoints: %w", err)
	}

	data := fiber.Map{
		"Metrics":       metrics,
		"Orders":        orders,
		"Records":       records,
		"Alerts":        alerts,
		"Checkpoints":   checkpoints,
		"DateFrom":      dateFrom,
		"DateTo":        dateTo,
		"ResidentID":    residentID,
		"StatusFilter":  status,
		"ServiceType":   serviceType,
		"HasFilters":    dateFrom != "" || dateTo != "" || residentID != "" || serviceType != "",
	}

	if c.Get("HX-Request") == "true" {
		return httpx.RenderPartial(c, "service_delivery/content_partial", data)
	}
	return httpx.RenderPage(c, "service_delivery/index", httpx.PageData{
		Title: "Service Delivery",
		User:  cu,
		Data:  data,
	})
}

// OrderDetail renders a single service order with its execution history.
func (h *ServiceDeliveryHandler) OrderDetail(c *fiber.Ctx) error {
	cu := authhandler.CurrentUser(c)
	id := c.Params("id")

	order, err := h.orderRepo.FindByID(id)
	if err != nil {
		return fmt.Errorf("service_delivery.OrderDetail: %w", err)
	}
	if order == nil {
		return fiber.NewError(fiber.StatusNotFound, "Service order not found.")
	}

	records, err := h.executionRepo.ListByOrderID(id)
	if err != nil {
		return fmt.Errorf("service_delivery.OrderDetail records: %w", err)
	}

	checkpoints, err := h.checkpointRepo.List(order.ResidentID, "", "")
	if err != nil {
		return fmt.Errorf("service_delivery.OrderDetail checkpoints: %w", err)
	}

	return httpx.RenderPage(c, "service_delivery/order_detail", httpx.PageData{
		Title: "Order — " + order.ResidentName,
		User:  cu,
		Data: fiber.Map{
			"Order":       order,
			"Records":     records,
			"Checkpoints": checkpoints,
		},
	})
}

// NewAlertForm renders the create-alert form.
func (h *ServiceDeliveryHandler) NewAlertForm(c *fiber.Ctx) error {
	cu := authhandler.CurrentUser(c)
	residents, err := h.admissionRepo.ListActiveResidents()
	if err != nil {
		return fmt.Errorf("service_delivery.NewAlertForm: %w", err)
	}
	return httpx.RenderPage(c, "service_delivery/alert_form", httpx.PageData{
		Title: "Record Alert",
		User:  cu,
		Data:  fiber.Map{"Residents": residents, "FormError": "", "Values": fiber.Map{}},
	})
}

// CreateAlert validates and creates a new alert event.
func (h *ServiceDeliveryHandler) CreateAlert(c *fiber.Ctx) error {
	cu := authhandler.CurrentUser(c)

	// Idempotency check
	iKey := idempotency.ExtractKey(c)
	if iKey != "" {
		if entry, _ := h.idempotency.Check(iKey); entry != nil {
			return c.Status(entry.ResponseStatus).Redirect(string(entry.ResponseBody))
		}
	}

	residentID := c.FormValue("resident_id")
	alertType := c.FormValue("alert_type")
	severity := c.FormValue("severity")
	message := c.FormValue("message")

	// Validate
	if err := validation.Required("resident_id", residentID); err != nil {
		return h.rerenderAlertForm(c, cu, "Resident is required.", residentID, alertType, severity, message)
	}
	if err := validation.Required("alert_type", alertType); err != nil {
		return h.rerenderAlertForm(c, cu, "Alert type is required.", residentID, alertType, severity, message)
	}
	if err := validation.Required("severity", severity); err != nil {
		return h.rerenderAlertForm(c, cu, "Severity is required.", residentID, alertType, severity, message)
	}
	if err := validation.Required("message", message); err != nil {
		return h.rerenderAlertForm(c, cu, "Message is required.", residentID, alertType, severity, message)
	}
	if err := validation.MaxLength("message", message, 500); err != nil {
		return h.rerenderAlertForm(c, cu, err.Error(), residentID, alertType, severity, message)
	}

	id, err := h.alertRepo.Create(sddomain.NewAlert{
		ResidentID:  residentID,
		AlertType:   alertType,
		Severity:    severity,
		Message:     message,
		TriggeredBy: cu.ID,
	})
	if err != nil {
		h.log.Error("create alert failed", "err", err)
		return h.rerenderAlertForm(c, cu, "Failed to save alert. Please try again.", residentID, alertType, severity, message)
	}

	h.auditSvc.Record(auditsvc.Entry{
		UserID:        cu.ID,
		Action:        "create",
		EntityType:    "alert_event",
		EntityID:      id,
		Details:       fmt.Sprintf(`{"alert_type":%q,"severity":%q,"resident_id":%q}`, alertType, severity, residentID),
		AfterSnapshot: audithelper.ToJSON(map[string]any{"id": id, "alert_type": alertType, "severity": severity, "resident_id": residentID, "message": message}),
		Outcome:       "success",
		IPAddress:     c.IP(),
		RequestID:     c.Get("X-Request-ID"),
	})

	if iKey != "" {
		_ = h.idempotency.Store(iKey, http.StatusFound, "/service-delivery")
	}

	return httpx.RedirectWithFlash(c, "/service-delivery", "Alert recorded successfully.")
}

// NewCheckpointForm renders the create-checkpoint form.
func (h *ServiceDeliveryHandler) NewCheckpointForm(c *fiber.Ctx) error {
	cu := authhandler.CurrentUser(c)
	residents, err := h.admissionRepo.ListActiveResidents()
	if err != nil {
		return fmt.Errorf("service_delivery.NewCheckpointForm: %w", err)
	}
	return httpx.RenderPage(c, "service_delivery/checkpoint_form", httpx.PageData{
		Title: "Record Care Checkpoint",
		User:  cu,
		Data:  fiber.Map{"Residents": residents, "FormError": "", "Values": fiber.Map{}},
	})
}

// CreateCheckpoint validates and creates a new care quality checkpoint.
func (h *ServiceDeliveryHandler) CreateCheckpoint(c *fiber.Ctx) error {
	cu := authhandler.CurrentUser(c)

	iKey := idempotency.ExtractKey(c)
	if iKey != "" {
		if entry, _ := h.idempotency.Check(iKey); entry != nil {
			return c.Status(entry.ResponseStatus).Redirect(string(entry.ResponseBody))
		}
	}

	residentID := c.FormValue("resident_id")
	cpType := c.FormValue("checkpoint_type")
	scoreStr := c.FormValue("score")
	notes := c.FormValue("notes")

	if err := validation.Required("resident_id", residentID); err != nil {
		return h.rerenderCheckpointForm(c, cu, "Resident is required.", residentID, cpType, scoreStr, notes)
	}
	if err := validation.Required("checkpoint_type", cpType); err != nil {
		return h.rerenderCheckpointForm(c, cu, "Checkpoint type is required.", residentID, cpType, scoreStr, notes)
	}
	score, err := strconv.Atoi(scoreStr)
	if err != nil || score < 1 || score > 5 {
		return h.rerenderCheckpointForm(c, cu, "Score must be 1–5.", residentID, cpType, scoreStr, notes)
	}

	id, err := h.checkpointRepo.Create(sddomain.NewCareCheckpoint{
		ResidentID:     residentID,
		CheckedBy:      cu.ID,
		CheckpointType: cpType,
		Score:          score,
		Notes:          notes,
	})
	if err != nil {
		h.log.Error("create checkpoint failed", "err", err)
		return h.rerenderCheckpointForm(c, cu, "Failed to save checkpoint. Please try again.", residentID, cpType, scoreStr, notes)
	}

	h.auditSvc.Record(auditsvc.Entry{
		UserID:        cu.ID,
		Action:        "create",
		EntityType:    "care_quality_checkpoint",
		EntityID:      id,
		Details:       fmt.Sprintf(`{"checkpoint_type":%q,"score":%d,"resident_id":%q}`, cpType, score, residentID),
		AfterSnapshot: audithelper.ToJSON(map[string]any{"id": id, "checkpoint_type": cpType, "score": score, "resident_id": residentID, "notes": notes}),
		Outcome:       "success",
		IPAddress:     c.IP(),
		RequestID:     c.Get("X-Request-ID"),
	})

	if iKey != "" {
		_ = h.idempotency.Store(iKey, http.StatusFound, "/service-delivery")
	}

	return httpx.RedirectWithFlash(c, "/service-delivery", "Care checkpoint recorded successfully.")
}

func (h *ServiceDeliveryHandler) rerenderAlertForm(c *fiber.Ctx, cu *httpx.CurrentUser, errMsg, residentID, alertType, severity, message string) error {
	residents, _ := h.admissionRepo.ListActiveResidents()
	return c.Status(fiber.StatusBadRequest).Render("service_delivery/alert_form", httpx.PageData{
		Title: "Record Alert",
		User:  cu,
		Data: fiber.Map{
			"Residents": residents,
			"FormError": errMsg,
			"Values": fiber.Map{
				"ResidentID": residentID,
				"AlertType":  alertType,
				"Severity":   severity,
				"Message":    message,
			},
		},
	}, "layouts/base")
}

func (h *ServiceDeliveryHandler) rerenderCheckpointForm(c *fiber.Ctx, cu *httpx.CurrentUser, errMsg, residentID, cpType, score, notes string) error {
	residents, _ := h.admissionRepo.ListActiveResidents()
	return c.Status(fiber.StatusBadRequest).Render("service_delivery/checkpoint_form", httpx.PageData{
		Title: "Record Care Checkpoint",
		User:  cu,
		Data: fiber.Map{
			"Residents": residents,
			"FormError": errMsg,
			"Values": fiber.Map{
				"ResidentID":     residentID,
				"CheckpointType": cpType,
				"Score":          score,
				"Notes":          notes,
			},
		},
	}, "layouts/base")
}
