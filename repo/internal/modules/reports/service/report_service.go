// Package service implements report definition management and on-demand report
// execution with CSV/XLSX output generation, audit logging, and job history tracking.
//
// Supported report types:
//   - occupancy         — current bed occupancy from beds + admissions
//   - service_delivery  — service orders with resident attribution
//   - finance           — payment records (reference numbers masked)
//   - audit             — audit log entries for compliance review
//
// Output files are written to outputPath. The caller is responsible for serving
// downloads via a path-safe handler.
package service

import (
	"database/sql"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"
	"time"

	auditsvc "careops/clinic/internal/modules/audit/service"
	audithelper "careops/clinic/internal/shared/audit"
	"careops/clinic/internal/modules/reports/domain"
	"careops/clinic/internal/modules/reports/repository"
	"careops/clinic/internal/platform/jobs"
	"careops/clinic/internal/shared/apperr"
	"careops/clinic/internal/shared/csvutil"
	"careops/clinic/internal/shared/idgen"
	"careops/clinic/internal/shared/xlsxutil"
)

// ReportService orchestrates report definition CRUD and on-demand execution.
type ReportService struct {
	db         *sql.DB
	repo       *repository.ReportRepo
	auditSvc   *auditsvc.AuditService
	jobWriter  *jobs.Writer
	outputPath string
	log        *slog.Logger
}

// New constructs a ReportService.
func New(
	db *sql.DB,
	repo *repository.ReportRepo,
	auditSvc *auditsvc.AuditService,
	jobWriter *jobs.Writer,
	outputPath string,
	log *slog.Logger,
) *ReportService {
	return &ReportService{
		db:         db,
		repo:       repo,
		auditSvc:   auditSvc,
		jobWriter:  jobWriter,
		outputPath: outputPath,
		log:        log,
	}
}

// CreateDefinition validates and persists a new scheduled report definition.
func (s *ReportService) CreateDefinition(cmd domain.CreateReportCmd, actorID, ipAddress, requestID string) (*domain.ScheduledReport, error) {
	if err := cmd.Validate(); err != nil {
		return nil, err
	}
	params := cmd.Parameters
	if params == "" {
		params = "{}"
	}
	sr := domain.ScheduledReport{
		ID:           idgen.New(),
		Name:         cmd.Name,
		ReportType:   cmd.ReportType,
		ScheduleCron: cmd.ScheduleCron,
		OutputFormat: cmd.OutputFormat,
		Parameters:   params,
		CreatedBy:    actorID,
	}
	if err := s.repo.Create(sr); err != nil {
		return nil, fmt.Errorf("reports: create definition: %w", err)
	}
	s.auditSvc.Record(auditsvc.Entry{
		UserID:        actorID,
		Action:        "create",
		EntityType:    "scheduled_reports",
		EntityID:      sr.ID,
		Details:       fmt.Sprintf("created report definition %q (%s/%s)", sr.Name, sr.ReportType, sr.OutputFormat),
		AfterSnapshot: audithelper.ToJSON(sr),
		IPAddress:     ipAddress,
		RequestID:     requestID,
		Outcome:       "success",
	})
	return &sr, nil
}

// UpdateDefinition changes name/cron/format/parameters on an existing definition.
// rowVersion enforces optimistic concurrency — returns apperr.ErrConflict when stale.
func (s *ReportService) UpdateDefinition(id string, cmd domain.CreateReportCmd, actorID, ipAddress, requestID string, rowVersion int) error {
	if err := cmd.Validate(); err != nil {
		return err
	}
	params := cmd.Parameters
	if params == "" {
		params = "{}"
	}

	// Capture before snapshot for audit trail.
	before, _ := s.repo.FindByID(id)
	beforeJSON := audithelper.ToJSON(before)

	matched, err := s.repo.Update(id, cmd.Name, cmd.ScheduleCron, cmd.OutputFormat, params, rowVersion)
	if err != nil {
		return fmt.Errorf("reports: update definition: %w", err)
	}
	if !matched {
		return &apperr.ConflictError{Entity: "scheduled_reports", ID: id}
	}
	s.auditSvc.Record(auditsvc.Entry{
		UserID:         actorID,
		Action:         "update",
		EntityType:     "scheduled_reports",
		EntityID:       id,
		Details:        fmt.Sprintf("updated report definition to %q (%s/%s)", cmd.Name, cmd.ReportType, cmd.OutputFormat),
		BeforeSnapshot: beforeJSON,
		AfterSnapshot:  audithelper.ToJSON(map[string]any{"name": cmd.Name, "report_type": cmd.ReportType, "output_format": cmd.OutputFormat, "schedule_cron": cmd.ScheduleCron}),
		IPAddress:      ipAddress,
		RequestID:      requestID,
		Outcome:        "success",
	})
	return nil
}

// ToggleActive flips the is_active flag on a report definition.
func (s *ReportService) ToggleActive(id, actorID, ipAddress, requestID string) error {
	sr, err := s.repo.FindByID(id)
	if err != nil {
		return fmt.Errorf("reports: find definition: %w", err)
	}
	if sr == nil {
		return fmt.Errorf("report definition not found")
	}
	if err := s.repo.SetActive(id, !sr.IsActive); err != nil {
		return fmt.Errorf("reports: toggle active: %w", err)
	}
	state := "activated"
	if sr.IsActive {
		state = "deactivated"
	}
	s.auditSvc.Record(auditsvc.Entry{
		UserID:         actorID,
		Action:         "update",
		EntityType:     "scheduled_reports",
		EntityID:       id,
		Details:        fmt.Sprintf("%s report definition %q", state, sr.Name),
		BeforeSnapshot: audithelper.ToJSON(map[string]any{"id": id, "name": sr.Name, "is_active": sr.IsActive}),
		AfterSnapshot:  audithelper.ToJSON(map[string]any{"id": id, "name": sr.Name, "is_active": !sr.IsActive}),
		IPAddress:      ipAddress,
		RequestID:      requestID,
		Outcome:        "success",
	})
	return nil
}

// RunReport executes the named report definition, writes the output file to
// outputPath, and records audit + job_history entries.
func (s *ReportService) RunReport(definitionID, actorID, ipAddress, requestID string) (*domain.ReportRun, error) {
	sr, err := s.repo.FindByID(definitionID)
	if err != nil {
		return nil, fmt.Errorf("reports: find definition: %w", err)
	}
	if sr == nil {
		return nil, fmt.Errorf("report definition not found")
	}

	start := time.Now()
	runID := idgen.New()
	run := domain.ReportRun{
		ID:                runID,
		ScheduledReportID: definitionID,
		ReportType:        sr.ReportType,
		TriggeredBy:       actorID,
		Parameters:        sr.Parameters,
		OutputFormat:      sr.OutputFormat,
		Status:            "pending",
	}
	if err := s.repo.CreateRun(run); err != nil {
		return nil, fmt.Errorf("reports: create run record: %w", err)
	}
	_ = s.repo.UpdateRun(runID, "running", "", "")

	headers, rows, genErr := s.generateData(sr)
	if genErr != nil {
		_ = s.repo.UpdateRun(runID, "failed", "", genErr.Error())
		_ = s.jobWriter.Record(jobs.Entry{
			JobName:       "report_run",
			JobType:       "on_demand",
			Status:        jobs.StatusFailed,
			DurationMs:    time.Since(start).Milliseconds(),
			Detail:        fmt.Sprintf("Report %q (%s) FAILED: %v", sr.Name, sr.ReportType, genErr),
			Actor:         actorID,
			CorrelationID: requestID,
		})
		return nil, genErr
	}

	ts := time.Now().UTC().Format("20060102_150405")
	fileName := fmt.Sprintf("report_%s_%s.%s", sr.ReportType, ts, sr.OutputFormat)
	outPath := filepath.Join(s.outputPath, fileName)

	var writeErr error
	switch sr.OutputFormat {
	case "xlsx":
		sheetTitle := sr.ReportType + "_report"
		writeErr = xlsxutil.WriteFile(outPath, sheetTitle, headers, rows)
	default:
		writeErr = csvutil.WriteFile(outPath, headers, rows)
	}
	if writeErr != nil {
		_ = s.repo.UpdateRun(runID, "failed", "", writeErr.Error())
		return nil, fmt.Errorf("reports: write output: %w", writeErr)
	}

	_ = s.repo.UpdateRun(runID, "complete", outPath, "")
	durationMs := time.Since(start).Milliseconds()

	s.auditSvc.Record(auditsvc.Entry{
		UserID:        actorID,
		Action:        "create",
		EntityType:    "report_runs",
		EntityID:      runID,
		Details:       fmt.Sprintf("ran report %q (%s) → %s (%d rows)", sr.Name, sr.ReportType, fileName, len(rows)),
		AfterSnapshot: audithelper.ToJSON(map[string]any{"id": runID, "report_type": sr.ReportType, "output_format": sr.OutputFormat, "row_count": len(rows), "file_name": fileName}),
		IPAddress:     ipAddress,
		RequestID:     requestID,
		Outcome:       "success",
	})
	_ = s.jobWriter.Record(jobs.Entry{
		JobName:       "report_run",
		JobType:       "on_demand",
		Status:        jobs.StatusSuccess,
		DurationMs:    durationMs,
		Detail:        fmt.Sprintf("Report %q (%s/%s) complete — %d rows → %s", sr.Name, sr.ReportType, sr.OutputFormat, len(rows), fileName),
		Actor:         actorID,
		CorrelationID: requestID,
	})

	return s.repo.FindRun(runID)
}

// ListDefinitions returns all report definitions ordered by name.
func (s *ReportService) ListDefinitions() ([]domain.ScheduledReport, error) {
	return s.repo.List()
}

// ListRuns returns the most recent run records with user display names.
func (s *ReportService) ListRuns(limit int) ([]domain.ReportRun, error) {
	return s.repo.ListRuns(limit)
}

// FindDefinition returns a single definition by ID, or (nil, nil) if missing.
func (s *ReportService) FindDefinition(id string) (*domain.ScheduledReport, error) {
	return s.repo.FindByID(id)
}

// FindRun returns a single run record by ID.
func (s *ReportService) FindRun(id string) (*domain.ReportRun, error) {
	return s.repo.FindRun(id)
}

// — data generators ——————————————————————————————————————————————————————

func (s *ReportService) generateData(sr *domain.ScheduledReport) (headers []string, rows [][]string, err error) {
	p := sr.ParseParams()
	switch sr.ReportType {
	case "occupancy":
		return s.buildOccupancyData(p)
	case "service_delivery":
		return s.buildServiceDeliveryData(p)
	case "finance":
		return s.buildFinanceData(p)
	case "audit":
		return s.buildAuditData(p)
	default:
		return nil, nil, fmt.Errorf("unknown report type %q", sr.ReportType)
	}
}

func (s *ReportService) buildOccupancyData(p domain.ReportParams) ([]string, [][]string, error) {
	headers := []string{"bed_id", "ward", "room_number", "bed_label", "bed_type", "status", "resident_name", "admission_date"}

	var args []any
	var where []string
	where = append(where, "b.is_active=1")
	if p.ResidentID != "" {
		where = append(where, "r.id = ?")
		args = append(args, p.ResidentID)
	}

	q := `SELECT b.id, b.ward, b.room_number, b.bed_label, b.bed_type,
		       CASE WHEN a.id IS NOT NULL THEN 'occupied' ELSE 'vacant' END,
		       COALESCE(r.first_name||' '||r.last_name, ''),
		       COALESCE(a.admitted_at, '')
		FROM beds b
		LEFT JOIN admissions a ON a.bed_id = b.id AND a.discharged_at IS NULL
		LEFT JOIN residents r ON r.id = a.resident_id`
	if len(where) > 0 {
		q += " WHERE " + strings.Join(where, " AND ")
	}
	q += " ORDER BY b.ward, b.room_number, b.bed_label"

	dbRows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, nil, fmt.Errorf("report occupancy: %w", err)
	}
	defer dbRows.Close()
	var rows [][]string
	for dbRows.Next() {
		var bedID, ward, roomNum, bedLabel, bedType, status, resident, admDate string
		if err := dbRows.Scan(&bedID, &ward, &roomNum, &bedLabel, &bedType, &status, &resident, &admDate); err != nil {
			return nil, nil, err
		}
		rows = append(rows, []string{bedID, ward, roomNum, bedLabel, bedType, status, resident, admDate})
	}
	return headers, rows, dbRows.Err()
}

func (s *ReportService) buildServiceDeliveryData(p domain.ReportParams) ([]string, [][]string, error) {
	headers := []string{"order_id", "resident_name", "service_type", "status", "start_date", "end_date"}

	var args []any
	var where []string
	if p.ResidentID != "" {
		where = append(where, "so.resident_id = ?")
		args = append(args, p.ResidentID)
	}
	if p.DateFrom != "" {
		where = append(where, "so.start_date >= ?")
		args = append(args, p.DateFrom)
	}
	if p.DateTo != "" {
		where = append(where, "so.start_date <= ?")
		args = append(args, p.DateTo)
	}

	q := `SELECT so.id, COALESCE(r.first_name||' '||r.last_name,''),
		       so.service_type, so.status,
		       COALESCE(so.start_date,''), COALESCE(so.end_date,'')
		FROM service_orders so
		LEFT JOIN residents r ON r.id = so.resident_id`
	if len(where) > 0 {
		q += " WHERE " + strings.Join(where, " AND ")
	}
	q += " ORDER BY so.start_date DESC LIMIT 5000"

	dbRows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, nil, fmt.Errorf("report service_delivery: %w", err)
	}
	defer dbRows.Close()
	var rows [][]string
	for dbRows.Next() {
		var id, resident, serviceType, status, startDate, endDate string
		if err := dbRows.Scan(&id, &resident, &serviceType, &status, &startDate, &endDate); err != nil {
			return nil, nil, err
		}
		rows = append(rows, []string{id, resident, serviceType, status, startDate, endDate})
	}
	return headers, rows, dbRows.Err()
}

func (s *ReportService) buildFinanceData(p domain.ReportParams) ([]string, [][]string, error) {
	headers := []string{"payment_id", "resident_name", "amount_dollars", "payment_method", "currency", "description", "payment_date"}

	var args []any
	var where []string
	if p.ResidentID != "" {
		where = append(where, "p.resident_id = ?")
		args = append(args, p.ResidentID)
	}
	if p.DateFrom != "" {
		where = append(where, "p.created_at >= ?")
		args = append(args, p.DateFrom)
	}
	if p.DateTo != "" {
		where = append(where, "p.created_at <= ?")
		args = append(args, p.DateTo+"T23:59:59Z")
	}

	q := `SELECT p.id, COALESCE(r.first_name||' '||r.last_name,''),
		       CAST(p.amount_cents AS REAL)/100.0,
		       p.payment_method, p.currency, p.description, p.created_at
		FROM payments p
		LEFT JOIN residents r ON r.id = p.resident_id`
	if len(where) > 0 {
		q += " WHERE " + strings.Join(where, " AND ")
	}
	q += " ORDER BY p.created_at DESC LIMIT 5000"

	dbRows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, nil, fmt.Errorf("report finance: %w", err)
	}
	defer dbRows.Close()
	var rows [][]string
	for dbRows.Next() {
		var id, resident string
		var amount float64
		var method, currency, description, createdAt string
		if err := dbRows.Scan(&id, &resident, &amount, &method, &currency, &description, &createdAt); err != nil {
			return nil, nil, err
		}
		rows = append(rows, []string{id, resident, fmt.Sprintf("%.2f", amount), method, currency, description, createdAt})
	}
	return headers, rows, dbRows.Err()
}

func (s *ReportService) buildAuditData(p domain.ReportParams) ([]string, [][]string, error) {
	headers := []string{"id", "user_id", "action", "entity_type", "entity_id", "details", "outcome", "occurred_at"}

	var args []any
	var where []string
	if p.UserID != "" {
		where = append(where, "user_id = ?")
		args = append(args, p.UserID)
	}
	if p.EntityType != "" {
		where = append(where, "entity_type = ?")
		args = append(args, p.EntityType)
	}
	if p.DateFrom != "" {
		where = append(where, "occurred_at >= ?")
		args = append(args, p.DateFrom)
	}
	if p.DateTo != "" {
		where = append(where, "occurred_at <= ?")
		args = append(args, p.DateTo+"T23:59:59Z")
	}

	q := `SELECT id, user_id, action, entity_type, entity_id, details, outcome, occurred_at FROM audit_logs`
	if len(where) > 0 {
		q += " WHERE " + strings.Join(where, " AND ")
	}
	q += " ORDER BY occurred_at DESC LIMIT 5000"

	dbRows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, nil, fmt.Errorf("report audit: %w", err)
	}
	defer dbRows.Close()
	var rows [][]string
	for dbRows.Next() {
		var id, userID, action, entityType, entityID, details, outcome, occurredAt string
		if err := dbRows.Scan(&id, &userID, &action, &entityType, &entityID, &details, &outcome, &occurredAt); err != nil {
			return nil, nil, err
		}
		rows = append(rows, []string{id, userID, action, entityType, entityID, details, outcome, occurredAt})
	}
	return headers, rows, dbRows.Err()
}

