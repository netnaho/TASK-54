package handler

import (
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	authhandler "careops/clinic/internal/modules/auth/handler"
	"careops/clinic/internal/modules/finance/domain"
	"careops/clinic/internal/modules/finance/repository"
	"careops/clinic/internal/modules/finance/service"
	"careops/clinic/internal/platform/idempotency"
	"careops/clinic/internal/shared/httpx"
	"github.com/gofiber/fiber/v2"
)

// FinanceHandler handles all finance-related HTTP endpoints.
type FinanceHandler struct {
	svc         *service.FinanceService
	payments    *repository.PaymentRepo
	refunds     *repository.RefundRepo
	shifts      *repository.ShiftRepo
	batches     *repository.BatchRepo
	idem        *idempotency.Service
	exportsPath string
	db          *sql.DB
	log         *slog.Logger
}

// NewFinanceHandler constructs the handler. The exportsPath is used by the
// download endpoint to serve generated files safely.
func NewFinanceHandler(
	svc *service.FinanceService,
	payments *repository.PaymentRepo,
	refunds *repository.RefundRepo,
	shifts *repository.ShiftRepo,
	batches *repository.BatchRepo,
	idem *idempotency.Service,
	exportsPath string,
	db *sql.DB,
	log *slog.Logger,
) *FinanceHandler {
	return &FinanceHandler{
		svc:         svc,
		payments:    payments,
		refunds:     refunds,
		shifts:      shifts,
		batches:     batches,
		idem:        idem,
		exportsPath: exportsPath,
		db:          db,
		log:         log,
	}
}

// ————————————————————————————————————————————————————————————————————————————
// Index — overview dashboard
// ————————————————————————————————————————————————————————————————————————————

// Index renders /finance.
func (h *FinanceHandler) Index(c *fiber.Ctx) error {
	shifts, err := h.shifts.List(10)
	if err != nil {
		return err
	}
	payments, err := h.payments.List(repository.PaymentFilter{Limit: 50})
	if err != nil {
		return err
	}
	batches, err := h.batches.List(10)
	if err != nil {
		return err
	}

	var totalCents int
	for _, p := range payments {
		totalCents += p.AmountCents
	}

	return httpx.RenderPage(c, "finance/index", httpx.PageData{
		Title: "Finance & Payments",
		User:  authhandler.CurrentUser(c),
		Data: fiber.Map{
			"Shifts":          shifts,
			"Payments":        payments,
			"Batches":         batches,
			"TotalCents":      totalCents,
			"TotalDollars":    fmt.Sprintf("$%d.%02d", totalCents/100, totalCents%100),
			"PaymentMethods":  paymentMethodOptions(),
			"ShiftNames":      shiftNameOptions(),
		},
	})
}

// ————————————————————————————————————————————————————————————————————————————
// Payment recording
// ————————————————————————————————————————————————————————————————————————————

// PaymentNew renders the new payment form at GET /finance/payments/new.
func (h *FinanceHandler) PaymentNew(c *fiber.Ctx) error {
	residents, err := h.loadResidents()
	if err != nil {
		return err
	}
	shifts, err := h.shifts.List(20)
	if err != nil {
		return err
	}
	return httpx.RenderPage(c, "finance/payment_form", httpx.PageData{
		Title: "Record Payment",
		User:  authhandler.CurrentUser(c),
		Data: fiber.Map{
			"Residents":      residents,
			"OpenShifts":     openShifts(shifts),
			"PaymentMethods": paymentMethodOptions(),
			"ShiftNames":     shiftNameOptions(),
		},
	})
}

// PaymentCreate handles POST /finance/payments.
func (h *FinanceHandler) PaymentCreate(c *fiber.Ctx) error {
	cu := authhandler.CurrentUser(c)
	idemKey := idempotency.ExtractKey(c)

	cmd := domain.CreatePayment{
		ResidentID:      strings.TrimSpace(c.FormValue("resident_id")),
		ShiftID:         strings.TrimSpace(c.FormValue("shift_id")),
		PaymentMethod:   strings.TrimSpace(c.FormValue("payment_method")),
		AmountDollarsIn: strings.TrimSpace(c.FormValue("amount")),
		ReferenceNumber: strings.TrimSpace(c.FormValue("reference_number")),
		Description:     strings.TrimSpace(c.FormValue("description")),
		Currency:        "USD",
	}

	p, err := h.svc.RecordPayment(cmd, cu.ID, idemKey, c.IP(), c.Get("X-Request-ID"))
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}
	return c.Redirect("/finance/payments/" + p.ID)
}

// PaymentDetail renders GET /finance/payments/:id with full reference number
// decryption for finance users.
func (h *FinanceHandler) PaymentDetail(c *fiber.Ctx) error {
	p, err := h.payments.FindByID(c.Params("id"))
	if err == sql.ErrNoRows {
		return fiber.ErrNotFound
	}
	if err != nil {
		return err
	}

	// Decrypt the reference number for the finance view.
	fullRef := h.svc.DecryptReference(p)

	refunds, err := h.refunds.ListByPayment(p.ID)
	if err != nil {
		return err
	}

	return httpx.RenderPage(c, "finance/payment_detail", httpx.PageData{
		Title: "Payment — " + p.AmountDollars(),
		User:  authhandler.CurrentUser(c),
		Data: fiber.Map{
			"Payment":    p,
			"FullRef":    fullRef,
			"Refunds":    refunds,
		},
	})
}

// ————————————————————————————————————————————————————————————————————————————
// Refunds
// ————————————————————————————————————————————————————————————————————————————

// RefundNew renders GET /finance/payments/:id/refund.
func (h *FinanceHandler) RefundNew(c *fiber.Ctx) error {
	p, err := h.payments.FindByID(c.Params("id"))
	if err == sql.ErrNoRows {
		return fiber.ErrNotFound
	}
	if err != nil {
		return err
	}
	return httpx.RenderPage(c, "finance/refund_form", httpx.PageData{
		Title: "Issue Refund",
		User:  authhandler.CurrentUser(c),
		Data:  fiber.Map{"Payment": p},
	})
}

// RefundCreate handles POST /finance/payments/:id/refund.
func (h *FinanceHandler) RefundCreate(c *fiber.Ctx) error {
	cu := authhandler.CurrentUser(c)
	idemKey := idempotency.ExtractKey(c)

	cmd := domain.CreateRefund{
		PaymentID:       c.Params("id"),
		AmountDollarsIn: strings.TrimSpace(c.FormValue("amount")),
		Reason:          strings.TrimSpace(c.FormValue("reason")),
	}

	rf, err := h.svc.ProcessRefund(cmd, cu.ID, idemKey, c.IP(), c.Get("X-Request-ID"))
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}
	return c.Redirect("/finance/payments/" + rf.PaymentID)
}

// ————————————————————————————————————————————————————————————————————————————
// Shift management
// ————————————————————————————————————————————————————————————————————————————

// ShiftList renders GET /finance/shifts.
func (h *FinanceHandler) ShiftList(c *fiber.Ctx) error {
	shifts, err := h.shifts.List(50)
	if err != nil {
		return err
	}
	return httpx.RenderPage(c, "finance/shift_list", httpx.PageData{
		Title: "Settlement Shifts",
		User:  authhandler.CurrentUser(c),
		Data:  fiber.Map{"Shifts": shifts, "ShiftNames": shiftNameOptions()},
	})
}

// ShiftOpen handles POST /finance/shifts (opens a new shift).
func (h *FinanceHandler) ShiftOpen(c *fiber.Ctx) error {
	cu := authhandler.CurrentUser(c)
	idemKey := idempotency.ExtractKey(c)
	date := strings.TrimSpace(c.FormValue("shift_date"))
	shiftName := strings.TrimSpace(c.FormValue("shift_name"))

	if date == "" {
		date = time.Now().UTC().Format("2006-01-02")
	}

	shift, err := h.svc.GetOrOpenShift(date, shiftName, cu.ID, idemKey, c.IP(), c.Get("X-Request-ID"))
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}
	return c.Redirect("/finance/shifts/" + shift.ID)
}

// ShiftDetail renders GET /finance/shifts/:id.
func (h *FinanceHandler) ShiftDetail(c *fiber.Ctx) error {
	shift, err := h.shifts.FindByID(c.Params("id"))
	if err == sql.ErrNoRows {
		return fiber.ErrNotFound
	}
	if err != nil {
		return err
	}
	payments, err := h.payments.List(repository.PaymentFilter{ShiftID: shift.ID})
	if err != nil {
		return err
	}
	return httpx.RenderPage(c, "finance/shift_detail", httpx.PageData{
		Title: fmt.Sprintf("Shift — %s %s", shift.ShiftLabel(), shift.ShiftDate),
		User:  authhandler.CurrentUser(c),
		Data: fiber.Map{
			"Shift":    shift,
			"Payments": payments,
		},
	})
}

// ShiftClose handles POST /finance/shifts/:id/close.
func (h *FinanceHandler) ShiftClose(c *fiber.Ctx) error {
	cu := authhandler.CurrentUser(c)
	idemKey := idempotency.ExtractKey(c)
	cmd := domain.CloseShiftCmd{
		ShiftID:             c.Params("id"),
		ReconciliationNotes: strings.TrimSpace(c.FormValue("reconciliation_notes")),
	}
	closed, err := h.svc.CloseShift(cmd, cu.ID, idemKey, c.IP(), c.Get("X-Request-ID"))
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}
	return c.Redirect("/finance/shifts/" + closed.ID)
}

// ————————————————————————————————————————————————————————————————————————————
// Card batch import
// ————————————————————————————————————————————————————————————————————————————

// BatchImportNew renders GET /finance/batches/new.
func (h *FinanceHandler) BatchImportNew(c *fiber.Ctx) error {
	shifts, err := h.shifts.List(20)
	if err != nil {
		return err
	}
	return httpx.RenderPage(c, "finance/batch_import", httpx.PageData{
		Title: "Import Card-Terminal Batch",
		User:  authhandler.CurrentUser(c),
		Data:  fiber.Map{"OpenShifts": openShifts(shifts)},
	})
}

// BatchImportSubmit handles POST /finance/batches.
func (h *FinanceHandler) BatchImportSubmit(c *fiber.Ctx) error {
	cu := authhandler.CurrentUser(c)
	idemKey := idempotency.ExtractKey(c)
	shiftID := strings.TrimSpace(c.FormValue("shift_id"))

	fh, err := c.FormFile("batch_file")
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "batch_file is required")
	}
	// Safety: reject files larger than 10 MB.
	if fh.Size > 10*1024*1024 {
		return fiber.NewError(fiber.StatusBadRequest, "file too large (max 10 MB)")
	}

	f, err := fh.Open()
	if err != nil {
		return fmt.Errorf("finance: open upload: %w", err)
	}
	defer f.Close()

	batch, err := h.svc.ImportCardBatch(fh.Filename, f, cu.ID, shiftID, idemKey, c.IP(), c.Get("X-Request-ID"))
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}
	return c.Redirect("/finance/batches/" + batch.ID)
}

// BatchDetail renders GET /finance/batches/:id.
func (h *FinanceHandler) BatchDetail(c *fiber.Ctx) error {
	b, err := h.batches.FindByID(c.Params("id"))
	if err == sql.ErrNoRows {
		return fiber.ErrNotFound
	}
	if err != nil {
		return err
	}
	// Show only payments imported by this specific batch.
	payments, err := h.payments.List(repository.PaymentFilter{BatchID: b.ID})
	if err != nil {
		h.log.Warn("batch detail: load payments failed", "batch_id", b.ID, "err", err)
		payments = nil
	}
	return httpx.RenderPage(c, "finance/batch_detail", httpx.PageData{
		Title: "Batch Import — " + b.FileName,
		User:  authhandler.CurrentUser(c),
		Data:  fiber.Map{"Batch": b, "Payments": payments},
	})
}

// ————————————————————————————————————————————————————————————————————————————
// Exports
// ————————————————————————————————————————————————————————————————————————————

// ExportNew renders GET /finance/exports/new.
func (h *FinanceHandler) ExportNew(c *fiber.Ctx) error {
	return httpx.RenderPage(c, "finance/export_form", httpx.PageData{
		Title: "Export Finance Report",
		User:  authhandler.CurrentUser(c),
		Data:  fiber.Map{},
	})
}

// ExportCreate handles POST /finance/exports.
func (h *FinanceHandler) ExportCreate(c *fiber.Ctx) error {
	cu := authhandler.CurrentUser(c)
	idemKey := idempotency.ExtractKey(c)

	if idemKey != "" {
		if entry, _ := h.idem.Check(idemKey); entry != nil {
			return c.Redirect("/finance/exports/download?file=" + entry.ResponseBody)
		}
	}

	params := domain.ExportParams{
		ReportType: strings.TrimSpace(c.FormValue("report_type")),
		DateFrom:   strings.TrimSpace(c.FormValue("date_from")),
		DateTo:     strings.TrimSpace(c.FormValue("date_to")),
		ShiftID:    strings.TrimSpace(c.FormValue("shift_id")),
		Format:     strings.TrimSpace(c.FormValue("format")),
	}
	if params.Format == "" {
		params.Format = "csv"
	}

	outPath, err := h.svc.ExportReport(params, cu.ID, c.IP(), c.Get("X-Request-ID"))
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}

	if idemKey != "" {
		_ = h.idem.Store(idemKey, 302, filepath.Base(outPath))
	}
	return c.Redirect("/finance/exports/download?file=" + filepath.Base(outPath))
}

// ExportDownload serves GET /finance/exports/download?file=<name>.
// Path traversal is blocked by filepath.Base and by verifying the file is
// under the configured exports directory.
func (h *FinanceHandler) ExportDownload(c *fiber.Ctx) error {
	name := filepath.Base(c.Query("file"))
	if name == "" || name == "." {
		return fiber.ErrBadRequest
	}
	fullPath := filepath.Join(h.exportsPath, name)
	// Ensure the resolved path is still under exportsPath.
	rel, err := filepath.Rel(h.exportsPath, fullPath)
	if err != nil || strings.HasPrefix(rel, "..") {
		return fiber.ErrBadRequest
	}
	if _, err := os.Stat(fullPath); err != nil {
		return fiber.ErrNotFound
	}

	contentType := "text/csv; charset=utf-8"
	if strings.HasSuffix(name, ".xlsx") {
		contentType = "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
	}
	data, err := os.ReadFile(fullPath)
	if err != nil {
		return err
	}
	c.Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, name))
	return c.Status(fiber.StatusOK).Type(contentType).Send(data)
}

// ————————————————————————————————————————————————————————————————————————————
// Helpers
// ————————————————————————————————————————————————————————————————————————————

type residentRow struct {
	ID   string
	Name string
	MRN  string
}

func (h *FinanceHandler) loadResidents() ([]residentRow, error) {
	rows, err := h.db.Query(`
		SELECT id, first_name || ' ' || last_name, mrn
		FROM residents ORDER BY last_name, first_name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var res []residentRow
	for rows.Next() {
		var r residentRow
		if err := rows.Scan(&r.ID, &r.Name, &r.MRN); err != nil {
			return nil, err
		}
		res = append(res, r)
	}
	return res, rows.Err()
}

func openShifts(all []domain.SettlementShift) []domain.SettlementShift {
	var open []domain.SettlementShift
	for _, s := range all {
		if s.Status == "open" {
			open = append(open, s)
		}
	}
	return open
}

type option struct{ Value, Label string }

func paymentMethodOptions() []option {
	return []option{
		{domain.MethodCash, "Cash"},
		{domain.MethodCheck, "Check"},
		{domain.MethodFacilityAccount, "Facility Charge Account"},
		{domain.MethodCard, "Card (terminal)"},
		{domain.MethodInsurance, "Insurance"},
		{domain.MethodBankTransfer, "Bank Transfer"},
	}
}

func shiftNameOptions() []option {
	return []option{
		{domain.ShiftMorning, "Morning (07:00–15:00)"},
		{domain.ShiftAfternoon, "Afternoon (15:00–23:00)"},
		{domain.ShiftNight, "Night (23:00–07:00)"},
	}
}
