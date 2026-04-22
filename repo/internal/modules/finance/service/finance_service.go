// Package service implements the finance module's business logic.
//
// All write operations are idempotent (supply an idempotency key from the
// form's _idempotency_key field), encrypted for sensitive reference numbers,
// and append an audit log entry.
package service

import (
	"encoding/csv"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	auditsvc "careops/clinic/internal/modules/audit/service"
	audithelper "careops/clinic/internal/shared/audit"
	"careops/clinic/internal/modules/finance/domain"
	"careops/clinic/internal/modules/finance/repository"
	"careops/clinic/internal/platform/idempotency"
	"careops/clinic/internal/platform/jobs"
	"careops/clinic/internal/shared/crypto"
	"careops/clinic/internal/shared/csvutil"
	"careops/clinic/internal/shared/timeutil"
	"careops/clinic/internal/shared/xlsxutil"
)

// FinanceService orchestrates payment recording, refunds, batch imports,
// shift-close settlement, and finance report exports.
type FinanceService struct {
	payments    *repository.PaymentRepo
	refunds     *repository.RefundRepo
	shifts      *repository.ShiftRepo
	batches     *repository.BatchRepo
	auditSvc    *auditsvc.AuditService
	idem        *idempotency.Service
	cipher      *crypto.Cipher
	isProd      bool
	exportsPath string
	batchesPath string
	jobWriter   *jobs.Writer
	log         *slog.Logger
}

// New constructs a FinanceService. isProd enforces that sensitive reference
// numbers cannot be written without an encryption key configured.
func New(
	payments *repository.PaymentRepo,
	refunds *repository.RefundRepo,
	shifts *repository.ShiftRepo,
	batches *repository.BatchRepo,
	auditSvc *auditsvc.AuditService,
	idem *idempotency.Service,
	cipher *crypto.Cipher,
	isProd bool,
	exportsPath string,
	jobWriter *jobs.Writer,
	log *slog.Logger,
) *FinanceService {
	return &FinanceService{
		payments:    payments,
		refunds:     refunds,
		shifts:      shifts,
		batches:     batches,
		auditSvc:    auditSvc,
		idem:        idem,
		cipher:      cipher,
		isProd:      isProd,
		exportsPath: exportsPath,
		batchesPath: filepath.Join(filepath.Dir(exportsPath), "batches"),
		jobWriter:   jobWriter,
		log:         log,
	}
}

// RecordPayment validates and persists a new payment.
// If idempotencyKey is non-empty and already resolved, the cached response is
// returned (as an error wrapping the payment ID) — callers should redirect to
// the existing payment.
func (s *FinanceService) RecordPayment(cmd domain.CreatePayment, actorID, idempotencyKey, ipAddress, requestID string) (*domain.Payment, error) {
	if err := cmd.Validate(); err != nil {
		return nil, err
	}

	// Idempotency check.
	if idempotencyKey != "" {
		if entry, _ := s.idem.Check(idempotencyKey); entry != nil {
			p, err := s.payments.FindByID(entry.ResponseBody)
			if err == nil {
				return p, nil
			}
		}
	}

	// In production, refuse sensitive writes when encryption is not configured.
	if s.isProd && cmd.ShouldEncryptReference() && !s.cipher.IsReady() {
		return nil, fmt.Errorf("encryption not configured; cannot record %s payment in production without FIELD_ENCRYPT_KEY", cmd.PaymentMethod)
	}

	// Encrypt the reference number if the payment method is sensitive.
	displayRef := cmd.ReferenceNumber
	encRef := ""
	if cmd.ShouldEncryptReference() {
		if s.cipher.IsReady() {
			enc, err := s.cipher.Encrypt(cmd.ReferenceNumber)
			if err != nil {
				s.log.Warn("reference encryption failed; storing masked value", "err", err)
			} else {
				encRef = enc
			}
		}
		displayRef = crypto.MaskString(cmd.ReferenceNumber)
	}

	p, err := s.payments.Create(
		cmd.ResidentID, cmd.ShiftID, "", cmd.PaymentMethod,
		cmd.AmountCents, cmd.Currency, displayRef, encRef,
		cmd.Description, actorID,
	)
	if err != nil {
		return nil, fmt.Errorf("finance: record payment: %w", err)
	}

	s.auditSvc.Record(auditsvc.Entry{
		UserID:        actorID,
		Action:        "create",
		EntityType:    "payments",
		EntityID:      p.ID,
		Details:       fmt.Sprintf("recorded %s payment %s for resident %s", p.PaymentMethod, p.AmountDollars(), p.ResidentID),
		AfterSnapshot: audithelper.ToJSON(map[string]any{"id": p.ID, "method": p.PaymentMethod, "amount_cents": p.AmountCents, "currency": p.Currency, "resident_id": p.ResidentID}),
		IPAddress:     ipAddress,
		RequestID:     requestID,
		Outcome:       "success",
	})

	if idempotencyKey != "" {
		_ = s.idem.Store(idempotencyKey, 302, p.ID)
	}
	return p, nil
}

// DecryptReference returns the raw (unmasked) reference number for a payment.
// Returns the stored display value if encryption is not configured.
func (s *FinanceService) DecryptReference(p *domain.Payment) string {
	if p.EncryptedReference == "" || !s.cipher.IsReady() {
		return p.ReferenceNumber
	}
	plain, err := s.cipher.Decrypt(p.EncryptedReference)
	if err != nil {
		s.log.Warn("reference decryption failed", "payment_id", p.ID)
		return p.ReferenceNumber
	}
	return plain
}

// ProcessRefund records a refund against an existing payment.
func (s *FinanceService) ProcessRefund(cmd domain.CreateRefund, actorID, idempotencyKey, ipAddress, requestID string) (*domain.Refund, error) {
	if err := cmd.Validate(); err != nil {
		return nil, err
	}

	if idempotencyKey != "" {
		if entry, _ := s.idem.Check(idempotencyKey); entry != nil {
			rf, err := s.refunds.FindByID(entry.ResponseBody)
			if err == nil {
				return rf, nil
			}
		}
	}

	// Verify the payment exists.
	p, err := s.payments.FindByID(cmd.PaymentID)
	if err != nil {
		return nil, fmt.Errorf("finance: payment %s not found: %w", cmd.PaymentID, err)
	}
	if p.Status == "reversed" {
		return nil, fmt.Errorf("payment %s is already fully reversed", cmd.PaymentID)
	}

	// Guard against refunding more than was paid.
	alreadyRefunded, err := s.refunds.TotalRefundedForPayment(cmd.PaymentID)
	if err != nil {
		return nil, fmt.Errorf("finance: compute existing refunds: %w", err)
	}
	if alreadyRefunded+cmd.AmountCents > p.AmountCents {
		return nil, fmt.Errorf(
			"refund of %s exceeds remaining balance %s",
			domain.CreateRefund{AmountCents: cmd.AmountCents}.AmountDollarsDisplay(),
			domain.CreateRefund{AmountCents: p.AmountCents - alreadyRefunded}.AmountDollarsDisplay(),
		)
	}

	rf, err := s.refunds.Create(cmd.PaymentID, actorID, cmd.AmountCents, cmd.Reason)
	if err != nil {
		return nil, fmt.Errorf("finance: create refund: %w", err)
	}

	// Mark payment as reversed if fully refunded.
	if alreadyRefunded+cmd.AmountCents == p.AmountCents {
		_ = s.payments.UpdateStatus(p.ID, "reversed", p.RowVersion)
	}

	s.auditSvc.Record(auditsvc.Entry{
		UserID:         actorID,
		Action:         "create",
		EntityType:     "refunds",
		EntityID:       rf.ID,
		Details:        fmt.Sprintf("refund %s on payment %s: %s", rf.ID, cmd.PaymentID, rf.AmountDollars()),
		BeforeSnapshot: audithelper.ToJSON(map[string]any{"payment_id": p.ID, "payment_status": p.Status, "amount_cents": p.AmountCents}),
		AfterSnapshot:  audithelper.ToJSON(rf),
		IPAddress:      ipAddress,
		RequestID:      requestID,
		Outcome:        "success",
	})

	if idempotencyKey != "" {
		_ = s.idem.Store(idempotencyKey, 302, rf.ID)
	}
	return rf, nil
}

// GetOrOpenShift returns the open shift for the given date+name, opening a new
// one if none exists yet. idempotencyKey deduplicates concurrent open requests.
func (s *FinanceService) GetOrOpenShift(date, shiftName, actorID, idempotencyKey, ipAddress, requestID string) (*domain.SettlementShift, error) {
	if !domain.ValidShiftNames[shiftName] {
		return nil, fmt.Errorf("invalid shift name %q", shiftName)
	}

	if idempotencyKey != "" {
		if entry, _ := s.idem.Check(idempotencyKey); entry != nil {
			if shift, err := s.shifts.FindByID(entry.ResponseBody); err == nil {
				return shift, nil
			}
		}
	}

	existing, err := s.shifts.FindOpen(date, shiftName)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return existing, nil
	}
	shift, err := s.shifts.Create(date, shiftName, actorID)
	if err != nil {
		return nil, fmt.Errorf("finance: open shift: %w", err)
	}

	if idempotencyKey != "" {
		_ = s.idem.Store(idempotencyKey, 302, shift.ID)
	}

	s.auditSvc.Record(auditsvc.Entry{
		UserID:        actorID,
		Action:        "create",
		EntityType:    "settlement_shifts",
		EntityID:      shift.ID,
		Details:       fmt.Sprintf("opened %s shift for %s", shiftName, date),
		AfterSnapshot: audithelper.ToJSON(shift),
		IPAddress:     ipAddress,
		RequestID:     requestID,
		Outcome:       "success",
	})
	return shift, nil
}

// CloseShift finalizes a shift and caches the total. idempotencyKey deduplicates concurrent close requests.
func (s *FinanceService) CloseShift(cmd domain.CloseShiftCmd, actorID, idempotencyKey, ipAddress, requestID string) (*domain.SettlementShift, error) {
	if idempotencyKey != "" {
		if entry, _ := s.idem.Check(idempotencyKey); entry != nil {
			if shift, err := s.shifts.FindByID(entry.ResponseBody); err == nil {
				return shift, nil
			}
		}
	}

	shift, err := s.shifts.FindByID(cmd.ShiftID)
	if err != nil {
		return nil, fmt.Errorf("finance: shift %s not found: %w", cmd.ShiftID, err)
	}
	if shift.Status != "open" {
		return nil, fmt.Errorf("shift %s is already %s; cannot close again", cmd.ShiftID, shift.Status)
	}

	total, err := s.shifts.SumPayments(cmd.ShiftID)
	if err != nil {
		return nil, fmt.Errorf("finance: compute shift total: %w", err)
	}

	notes := cmd.ReconciliationNotes
	if notes == "" {
		notes = fmt.Sprintf("Closed by %s. Total: %s.", actorID, domain.SettlementShift{TotalAmountCents: total}.TotalDollars())
	}

	if err := s.shifts.Close(cmd.ShiftID, actorID, total, notes); err != nil {
		return nil, fmt.Errorf("finance: close shift: %w", err)
	}

	closed, _ := s.shifts.FindByID(cmd.ShiftID)

	s.auditSvc.Record(auditsvc.Entry{
		UserID:         actorID,
		Action:         "update",
		EntityType:     "settlement_shifts",
		EntityID:       cmd.ShiftID,
		Details:        fmt.Sprintf("closed %s shift %s (%s). total: %s", shift.ShiftName, shift.ShiftDate, cmd.ShiftID, domain.SettlementShift{TotalAmountCents: total}.TotalDollars()),
		BeforeSnapshot: audithelper.ToJSON(shift),
		AfterSnapshot:  audithelper.ToJSON(closed),
		IPAddress:      ipAddress,
		RequestID:      requestID,
		Outcome:        "success",
	})

	_ = s.jobWriter.Record(jobs.Entry{
		JobName:       "shift_close",
		JobType:       "on_demand",
		Status:        jobs.StatusSuccess,
		Detail:        fmt.Sprintf("Shift %s (%s %s) closed. Total: %s across %d payments.", cmd.ShiftID, shift.ShiftName, shift.ShiftDate, domain.SettlementShift{TotalAmountCents: total}.TotalDollars(), shift.TotalPayments),
		Actor:         actorID,
		CorrelationID: requestID,
	})

	if idempotencyKey != "" {
		_ = s.idem.Store(idempotencyKey, 302, cmd.ShiftID)
	}

	return closed, nil
}

// ImportCardBatch reads a CSV file from reader, creates payment records for
// each line, and records batch statistics in job_history.
//
// Expected CSV format (with header row):
//
//	resident_mrn,amount_dollars,description,reference_number
//
// The CSV is written to batchesPath/<batchID>_<filename> before parsing so
// the file is preserved regardless of parse errors.
func (s *FinanceService) ImportCardBatch(
	fileName string, r io.Reader, importedBy, shiftID, idempotencyKey, ipAddress, requestID string,
) (*domain.CardBatch, error) {
	if idempotencyKey != "" {
		if entry, _ := s.idem.Check(idempotencyKey); entry != nil {
			if b, err := s.batches.FindByID(entry.ResponseBody); err == nil {
				return b, nil
			}
		}
	}

	// Persist the uploaded file.
	if err := os.MkdirAll(s.batchesPath, 0o755); err != nil {
		return nil, fmt.Errorf("finance: create batches dir: %w", err)
	}

	batch, err := s.batches.Create(fileName, "", importedBy, shiftID)
	if err != nil {
		return nil, fmt.Errorf("finance: create batch record: %w", err)
	}

	safeName := sanitizeFileName(fmt.Sprintf("%s_%s", batch.ID, fileName))
	filePath := filepath.Join(s.batchesPath, safeName)

	f, err := os.Create(filePath)
	if err != nil {
		_ = s.batches.SetResult(batch.ID, 0, 0, 0, "failed", fmt.Sprintf("create file: %v", err))
		return nil, fmt.Errorf("finance: save batch file: %w", err)
	}

	// Write CSV contents to disk while also keeping them in memory for parsing.
	var buf strings.Builder
	mw := io.MultiWriter(f, &buf)
	if _, err := io.Copy(mw, r); err != nil {
		f.Close()
		_ = s.batches.SetResult(batch.ID, 0, 0, 0, "failed", fmt.Sprintf("save file: %v", err))
		return nil, fmt.Errorf("finance: write batch file: %w", err)
	}
	f.Close()

	// Update the stored file path.
	_ = s.batches.SetResult(batch.ID, 0, 0, 0, "processing", "")

	// Parse CSV records.
	totalCount, successCount, errorCount, parseErr := s.processBatchCSV(
		strings.NewReader(buf.String()), batch.ID, shiftID, importedBy,
	)

	status := "complete"
	errDetail := ""
	if parseErr != nil {
		status = "failed"
		errDetail = parseErr.Error()
		errorCount = totalCount - successCount
	}

	_ = s.batches.SetResult(batch.ID, totalCount, successCount, errorCount, status, errDetail)

	s.auditSvc.Record(auditsvc.Entry{
		UserID:        importedBy,
		Action:        "create",
		EntityType:    "imported_card_batches",
		EntityID:      batch.ID,
		Details:       fmt.Sprintf("imported card batch %s: %d records, %d ok, %d errors", fileName, totalCount, successCount, errorCount),
		AfterSnapshot: audithelper.ToJSON(map[string]any{"batch_id": batch.ID, "file_name": fileName, "total": totalCount, "success": successCount, "errors": errorCount, "status": status}),
		IPAddress:     ipAddress,
		RequestID:     requestID,
		Outcome:       status,
	})

	_ = s.jobWriter.Record(jobs.Entry{
		JobName:       "card_batch_import",
		JobType:       "on_demand",
		Status:        status,
		Detail:        fmt.Sprintf("Batch %s: %d records, %d ok, %d errors. %s", fileName, totalCount, successCount, errorCount, errDetail),
		Actor:         importedBy,
		CorrelationID: requestID,
	})

	if idempotencyKey != "" {
		_ = s.idem.Store(idempotencyKey, 302, batch.ID)
	}

	return s.batches.FindByID(batch.ID)
}

// processBatchCSV reads the CSV and creates a payment for each valid row.
// Returns totalCount, successCount, errorCount, and the first fatal error.
func (s *FinanceService) processBatchCSV(r io.Reader, batchID, shiftID, actorID string) (total, success, errCount int, _ error) {
	cr := csv.NewReader(r)
	cr.TrimLeadingSpace = true

	// Read and validate header.
	header, err := cr.Read()
	if err != nil {
		return 0, 0, 0, fmt.Errorf("batch CSV: read header: %w", err)
	}
	idx := map[string]int{}
	for i, h := range header {
		idx[strings.ToLower(strings.TrimSpace(h))] = i
	}
	required := []string{"resident_mrn", "amount_dollars", "description"}
	for _, col := range required {
		if _, ok := idx[col]; !ok {
			return 0, 0, 0, fmt.Errorf("batch CSV: missing required column %q", col)
		}
	}

	// Resident MRN → ID lookup cache.
	mrnCache := map[string]string{}
	mrn2id := func(mrn string) (string, error) {
		if id, ok := mrnCache[mrn]; ok {
			return id, nil
		}
		var id string
		err := s.payments.DB().QueryRow(`SELECT id FROM residents WHERE mrn=?`, mrn).Scan(&id)
		if err != nil {
			return "", fmt.Errorf("resident MRN %q not found", mrn)
		}
		mrnCache[mrn] = id
		return id, nil
	}

	rowNum := 1
	for {
		row, err := cr.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			errCount++
			rowNum++
			continue
		}
		total++
		rowNum++

		mrn := strings.TrimSpace(row[idx["resident_mrn"]])
		amtStr := strings.TrimSpace(row[idx["amount_dollars"]])
		desc := strings.TrimSpace(row[idx["description"]])
		ref := ""
		if i, ok := idx["reference_number"]; ok && i < len(row) {
			ref = strings.TrimSpace(row[i])
		}

		residentID, err := mrn2id(mrn)
		if err != nil {
			errCount++
			s.log.Warn("batch import: skip row", "row", rowNum, "err", err)
			continue
		}

		cmd := domain.CreatePayment{
			ResidentID:      residentID,
			ShiftID:         shiftID,
			PaymentMethod:   domain.MethodCard,
			AmountDollarsIn: amtStr,
			ReferenceNumber: ref,
			Description:     desc,
		}
		if err := cmd.Validate(); err != nil {
			errCount++
			s.log.Warn("batch import: validation error", "row", rowNum, "err", err)
			continue
		}

		displayRef := cmd.ReferenceNumber
		encRef := ""
		if cmd.ShouldEncryptReference() && s.cipher.IsReady() {
			if enc, err2 := s.cipher.Encrypt(cmd.ReferenceNumber); err2 == nil {
				encRef = enc
				displayRef = crypto.MaskString(cmd.ReferenceNumber)
			}
		}

		_, err = s.payments.Create(
			cmd.ResidentID, cmd.ShiftID, batchID, cmd.PaymentMethod,
			cmd.AmountCents, "USD", displayRef, encRef, cmd.Description, actorID,
		)
		if err != nil {
			errCount++
			s.log.Warn("batch import: insert error", "row", rowNum, "err", err)
			continue
		}
		success++
	}
	return total, success, errCount, nil
}

// ExportReport generates a finance or occupancy report file in CSV or XLSX
// format and records the run in report_runs and job_history.
func (s *FinanceService) ExportReport(params domain.ExportParams, actorID, ipAddress, requestID string) (string, error) {
	if err := params.Validate(); err != nil {
		return "", err
	}

	var headers []string
	var rows [][]string
	var err error

	switch params.ReportType {
	case "finance":
		headers, rows, err = s.buildFinanceRows(params)
	case "occupancy":
		headers, rows, err = s.buildOccupancyRows(params)
	default:
		return "", fmt.Errorf("unknown report type %q", params.ReportType)
	}
	if err != nil {
		return "", fmt.Errorf("finance: build report data: %w", err)
	}

	ts := time.Now().UTC().Format("20060102_150405")
	fileName := fmt.Sprintf("%s_%s.%s", params.ReportType, ts, params.Format)
	outPath := filepath.Join(s.exportsPath, fileName)

	switch params.Format {
	case "csv":
		err = csvutil.WriteFile(outPath, headers, rows)
	case "xlsx":
		sheetTitle := strings.ToUpper(params.ReportType[:1]) + params.ReportType[1:] + " Report"
		err = xlsxutil.WriteFile(outPath, sheetTitle, headers, rows)
	}
	if err != nil {
		return "", fmt.Errorf("finance: write export file: %w", err)
	}

	s.auditSvc.Record(auditsvc.Entry{
		UserID:        actorID,
		Action:        "create",
		EntityType:    "report_runs",
		EntityID:      fileName,
		Details:       fmt.Sprintf("exported %s report as %s (%d rows)", params.ReportType, params.Format, len(rows)),
		AfterSnapshot: audithelper.ToJSON(map[string]any{"report_type": params.ReportType, "format": params.Format, "row_count": len(rows), "file_name": fileName}),
		IPAddress:     ipAddress,
		RequestID:     requestID,
		Outcome:       "success",
	})

	_ = s.jobWriter.Record(jobs.Entry{
		JobName:       "finance_export",
		JobType:       "on_demand",
		Status:        jobs.StatusSuccess,
		Detail:        fmt.Sprintf("%s report: %s (%d data rows)", params.ReportType, outPath, len(rows)),
		Actor:         actorID,
		CorrelationID: requestID,
	})

	return outPath, nil
}

// buildFinanceRows queries payments and returns header+rows for the finance report.
func (s *FinanceService) buildFinanceRows(p domain.ExportParams) ([]string, [][]string, error) {
	f := repository.PaymentFilter{
		ShiftID:  p.ShiftID,
		DateFrom: p.DateFrom,
		DateTo:   p.DateTo,
	}
	if f.DateFrom == "" && f.DateTo == "" && f.ShiftID == "" {
		f.DateFrom = timeutil.NowISO()[:10]
		f.DateTo = f.DateFrom
	}
	payments, err := s.payments.List(f)
	if err != nil {
		return nil, nil, err
	}

	headers := []string{
		"Payment ID", "Recorded At", "Resident", "Shift ID",
		"Method", "Amount", "Currency", "Reference (masked)", "Description", "Status",
	}
	var rows [][]string
	for _, pay := range payments {
		rows = append(rows, []string{
			pay.ID,
			pay.RecordedAt.Format("2006-01-02 15:04"),
			pay.ResidentName,
			pay.ShiftID,
			pay.PaymentMethod,
			pay.AmountDollars(),
			pay.Currency,
			pay.ReferenceNumber, // already masked in DB for sensitive methods
			pay.Description,
			pay.Status,
		})
	}
	return headers, rows, nil
}

// buildOccupancyRows queries occupancy snapshots for the date range.
func (s *FinanceService) buildOccupancyRows(p domain.ExportParams) ([]string, [][]string, error) {
	dateFrom := p.DateFrom
	dateTo := p.DateTo
	if dateFrom == "" {
		dateFrom = "2000-01-01"
	}
	if dateTo == "" {
		dateTo = timeutil.NowISO()[:10]
	}

	rows2, err := s.payments.DB().Query(`
		SELECT snapshot_date, total_beds, occupied_beds, available_beds, occupancy_rate
		FROM occupancy_snapshots
		WHERE snapshot_date >= ? AND snapshot_date <= ?
		ORDER BY snapshot_date`, dateFrom, dateTo)
	if err != nil {
		return nil, nil, err
	}
	defer rows2.Close()

	headers := []string{"Date", "Total Beds", "Occupied", "Available", "Occupancy %"}
	var rows [][]string
	for rows2.Next() {
		var date string
		var total, occupied, available int
		var rate float64
		if err := rows2.Scan(&date, &total, &occupied, &available, &rate); err != nil {
			return nil, nil, err
		}
		rows = append(rows, []string{
			date,
			fmt.Sprintf("%d", total),
			fmt.Sprintf("%d", occupied),
			fmt.Sprintf("%d", available),
			fmt.Sprintf("%.1f%%", rate*100),
		})
	}
	return headers, rows, rows2.Err()
}

// sanitizeFileName removes path traversal characters from a user-supplied name.
func sanitizeFileName(name string) string {
	name = filepath.Base(name)
	name = strings.ReplaceAll(name, "..", "")
	return name
}
