package repository

import (
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"time"

	"careops/clinic/internal/modules/reports/domain"
	"careops/clinic/internal/shared/timeutil"
)

// ReportRepo owns all SQL for scheduled_reports and report_runs.
type ReportRepo struct {
	db *sql.DB
}

// NewReportRepo constructs a ReportRepo.
func NewReportRepo(db *sql.DB) *ReportRepo { return &ReportRepo{db: db} }

// Create inserts a new scheduled report definition.
func (r *ReportRepo) Create(sr domain.ScheduledReport) error {
	_, err := r.db.Exec(`
		INSERT INTO scheduled_reports
			(id, name, report_type, schedule_cron, parameters, output_format,
			 recipients, is_active, created_by, created_at, updated_at, row_version)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,1)`,
		sr.ID, sr.Name, sr.ReportType, sr.ScheduleCron, sr.Parameters, sr.OutputFormat,
		"[]", 1, sr.CreatedBy, timeutil.NowISO(), timeutil.NowISO(),
	)
	return err
}

// Update applies field changes and increments row_version atomically.
// Returns (false, nil) when row_version no longer matches — optimistic lock conflict.
func (r *ReportRepo) Update(id, name, cron, outputFormat, params string, rowVersion int) (bool, error) {
	res, err := r.db.Exec(`
		UPDATE scheduled_reports
		SET name=?, schedule_cron=?, output_format=?, parameters=?,
		    updated_at=?, row_version=row_version+1
		WHERE id=? AND row_version=?`,
		name, cron, outputFormat, params, timeutil.NowISO(), id, rowVersion,
	)
	if err != nil {
		return false, fmt.Errorf("reports repo: update: %w", err)
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

// SetActive flips the is_active flag without touching row_version (non-content change).
func (r *ReportRepo) SetActive(id string, active bool) error {
	v := 0
	if active {
		v = 1
	}
	_, err := r.db.Exec(
		`UPDATE scheduled_reports SET is_active=?, updated_at=? WHERE id=?`,
		v, timeutil.NowISO(), id,
	)
	return err
}

// FindByID returns a single definition, or (nil, nil) if not found.
func (r *ReportRepo) FindByID(id string) (*domain.ScheduledReport, error) {
	row := r.db.QueryRow(`
		SELECT id, name, report_type, schedule_cron, parameters, output_format,
		       is_active, created_by, row_version, created_at, updated_at
		FROM scheduled_reports WHERE id=?`, id)
	s, err := scanReport(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return s, err
}

// List returns all definitions ordered by name.
func (r *ReportRepo) List() ([]domain.ScheduledReport, error) {
	rows, err := r.db.Query(`
		SELECT id, name, report_type, schedule_cron, parameters, output_format,
		       is_active, created_by, row_version, created_at, updated_at
		FROM scheduled_reports ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("reports repo: list: %w", err)
	}
	defer rows.Close()
	var items []domain.ScheduledReport
	for rows.Next() {
		s, err := scanReport(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, *s)
	}
	return items, rows.Err()
}

type rowScanner interface {
	Scan(...any) error
}

func scanReport(row rowScanner) (*domain.ScheduledReport, error) {
	var s domain.ScheduledReport
	var isActive int
	var ca, ua string
	if err := row.Scan(
		&s.ID, &s.Name, &s.ReportType, &s.ScheduleCron, &s.Parameters, &s.OutputFormat,
		&isActive, &s.CreatedBy, &s.RowVersion, &ca, &ua,
	); err != nil {
		return nil, err
	}
	s.IsActive = isActive == 1
	s.CreatedAt, _ = time.Parse(time.RFC3339, ca)
	s.UpdatedAt, _ = time.Parse(time.RFC3339, ua)
	return &s, nil
}

// ListActiveScheduled returns all active report definitions that have a non-empty
// schedule_cron — these are the candidates the scheduler evaluates each minute.
func (r *ReportRepo) ListActiveScheduled() ([]domain.ScheduledReport, error) {
	rows, err := r.db.Query(`
		SELECT id, name, report_type, schedule_cron, parameters, output_format,
		       is_active, created_by, row_version, created_at, updated_at
		FROM scheduled_reports
		WHERE is_active=1 AND schedule_cron != ''
		ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("reports repo: list active scheduled: %w", err)
	}
	defer rows.Close()
	var items []domain.ScheduledReport
	for rows.Next() {
		s, err := scanReport(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, *s)
	}
	return items, rows.Err()
}

// HasRunForWindow reports whether the scheduler already triggered a run for
// definitionID within [windowStart, windowEnd). Used for deduplication so that
// restarts or multiple ticks in the same minute window don't double-run a report.
func (r *ReportRepo) HasRunForWindow(defID string, windowStart, windowEnd time.Time) (bool, error) {
	var count int
	err := r.db.QueryRow(`
		SELECT COUNT(*) FROM report_runs
		WHERE scheduled_report_id = ?
		  AND triggered_by = 'scheduler'
		  AND created_at >= ? AND created_at < ?`,
		defID,
		windowStart.UTC().Format(time.RFC3339),
		windowEnd.UTC().Format(time.RFC3339),
	).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("reports repo: has run for window: %w", err)
	}
	return count > 0, nil
}

// CreateRun inserts a new report_runs row with status "pending".
func (r *ReportRepo) CreateRun(run domain.ReportRun) error {
	_, err := r.db.Exec(`
		INSERT INTO report_runs
			(id, scheduled_report_id, report_type, triggered_by,
			 parameters, status, output_path, error_message, output_format, created_at)
		VALUES (?,?,?,?,?,?,?,?,?,?)`,
		run.ID, run.ScheduledReportID, run.ReportType, run.TriggeredBy,
		run.Parameters, run.Status, "", "", run.OutputFormat, timeutil.NowISO(),
	)
	return err
}

// UpdateRun sets the status, output_path, and error_message.
// started_at is set on the first transition to "running" (using COALESCE to avoid overwrite).
// completed_at is set when status is "complete" or "failed".
func (r *ReportRepo) UpdateRun(id, status, outputPath, errMsg string) error {
	now := timeutil.NowISO()
	var startedAt, completedAt sql.NullString
	if status == "running" {
		startedAt = sql.NullString{Valid: true, String: now}
	}
	if status == "complete" || status == "failed" {
		completedAt = sql.NullString{Valid: true, String: now}
	}
	_, err := r.db.Exec(`
		UPDATE report_runs
		SET status=?, output_path=?, error_message=?,
		    started_at=COALESCE(NULLIF(started_at,''), ?),
		    completed_at=?
		WHERE id=?`,
		status, outputPath, errMsg, startedAt, completedAt, id,
	)
	return err
}

// FindRun returns a single run joined with the triggering user's display_name.
// Returns (nil, nil) when not found.
func (r *ReportRepo) FindRun(id string) (*domain.ReportRun, error) {
	row := r.db.QueryRow(`
		SELECT rr.id, COALESCE(rr.scheduled_report_id,''), rr.report_type,
		       COALESCE(rr.triggered_by,''), COALESCE(u.display_name,''),
		       rr.parameters, rr.output_path, rr.output_format,
		       rr.status, rr.error_message,
		       rr.started_at, rr.completed_at, rr.created_at
		FROM report_runs rr
		LEFT JOIN users u ON u.id = rr.triggered_by
		WHERE rr.id=?`, id)
	run, err := scanRun(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return run, err
}

// ListRuns returns the most recent runs with triggering user display names.
func (r *ReportRepo) ListRuns(limit int) ([]domain.ReportRun, error) {
	rows, err := r.db.Query(fmt.Sprintf(`
		SELECT rr.id, COALESCE(rr.scheduled_report_id,''), rr.report_type,
		       COALESCE(rr.triggered_by,''), COALESCE(u.display_name,''),
		       rr.parameters, rr.output_path, rr.output_format,
		       rr.status, rr.error_message,
		       rr.started_at, rr.completed_at, rr.created_at
		FROM report_runs rr
		LEFT JOIN users u ON u.id = rr.triggered_by
		ORDER BY rr.created_at DESC LIMIT %d`, limit))
	if err != nil {
		return nil, fmt.Errorf("reports repo: list runs: %w", err)
	}
	defer rows.Close()
	var items []domain.ReportRun
	for rows.Next() {
		run, err := scanRun(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, *run)
	}
	return items, rows.Err()
}

func scanRun(row rowScanner) (*domain.ReportRun, error) {
	var run domain.ReportRun
	var startedAt, completedAt sql.NullString
	var ca string
	if err := row.Scan(
		&run.ID, &run.ScheduledReportID, &run.ReportType,
		&run.TriggeredBy, &run.TriggeredByName,
		&run.Parameters, &run.OutputPath, &run.OutputFormat,
		&run.Status, &run.ErrorMessage,
		&startedAt, &completedAt, &ca,
	); err != nil {
		return nil, err
	}
	run.CreatedAt, _ = time.Parse(time.RFC3339, ca)
	if startedAt.Valid && startedAt.String != "" {
		t, _ := time.Parse(time.RFC3339, startedAt.String)
		run.StartedAt = &t
	}
	if completedAt.Valid && completedAt.String != "" {
		t, _ := time.Parse(time.RFC3339, completedAt.String)
		run.CompletedAt = &t
	}
	if run.OutputPath != "" {
		run.OutputFileName = filepath.Base(run.OutputPath)
	}
	return &run, nil
}
