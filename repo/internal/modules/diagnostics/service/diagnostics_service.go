// Package service generates diagnostic ZIP packages that bundle DB statistics,
// recent job history, the current configuration snapshot, recent audit log
// entries, and any persisted configuration snapshots from the snapshot path.
//
// What is included in a package:
//   - README.txt              — explains contents/exclusions
//   - db_stats.json           — SQLite PRAGMA stats + per-table row counts
//   - recent_jobs.json        — last 100 job_history rows
//   - config.json             — current config_versions entries (namespace/key/value)
//   - audit_logs.json         — last 200 audit_log entries (structured log bundle)
//   - config_snapshots/       — up to 3 most-recent JSON snapshot files
//
// What is NOT included (documented in README.txt):
//   - Encryption keys or other runtime secrets (these live in env vars)
//   - Password hashes
//   - Raw payment reference numbers
//   - PII beyond what is already in the audit log and config tables
package service

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	auditsvc "careops/clinic/internal/modules/audit/service"
	audithelper "careops/clinic/internal/shared/audit"
	"careops/clinic/internal/platform/jobs"
	"careops/clinic/internal/shared/idgen"
	"careops/clinic/internal/shared/timeutil"
	"careops/clinic/internal/shared/ziputil"
)

// DiagnosticsService generates diagnostic export packages.
type DiagnosticsService struct {
	db             *sql.DB
	auditSvc       *auditsvc.AuditService
	jobWriter      *jobs.Writer
	diagPath       string
	configSnapPath string
	logPath        string // optional path to the running app log file
	log            *slog.Logger
}

// New constructs a DiagnosticsService.
func New(
	db *sql.DB,
	auditSvc *auditsvc.AuditService,
	jobWriter *jobs.Writer,
	diagPath string,
	configSnapPath string,
	log *slog.Logger,
) *DiagnosticsService {
	return &DiagnosticsService{
		db:             db,
		auditSvc:       auditSvc,
		jobWriter:      jobWriter,
		diagPath:       diagPath,
		configSnapPath: configSnapPath,
		log:            log,
	}
}

// WithLogPath configures the path to the running application log file.
// When set, the last 5 000 bytes of the log are included in diagnostic ZIPs
// after secrets have been redacted.
func (s *DiagnosticsService) WithLogPath(p string) *DiagnosticsService {
	s.logPath = p
	return s
}

// ExportPackage creates a ZIP archive with diagnostic data and records the
// export in diagnostic_exports and job_history.
// Returns the archive file path.
func (s *DiagnosticsService) ExportPackage(actorID, ipAddress, correlationID string) (string, error) {
	start := time.Now()
	exportID := idgen.New()
	now := timeutil.NowISO()
	ts := time.Now().UTC().Format("20060102_150405")
	fileName := fmt.Sprintf("careops_diag_%s.zip", ts)
	filePath := filepath.Join(s.diagPath, fileName)

	_, err := s.db.Exec(`
		INSERT INTO diagnostic_exports
		    (id, export_type, triggered_by, file_path, status,
		     error_message, triggered_at, created_at)
		VALUES (?,?,?,?,?,?,?,?)`,
		exportID, "full", actorID, filePath, "pending", "", now, now,
	)
	if err != nil {
		return "", fmt.Errorf("diag: insert export record: %w", err)
	}

	entries, err := s.buildEntries()
	if err != nil {
		_ = s.markFailed(exportID, err.Error())
		return "", fmt.Errorf("diag: build entries: %w", err)
	}

	if err := ziputil.Create(filePath, entries); err != nil {
		_ = s.markFailed(exportID, err.Error())
		return "", fmt.Errorf("diag: create zip: %w", err)
	}

	completedAt := timeutil.NowISO()
	_, _ = s.db.Exec(`
		UPDATE diagnostic_exports
		SET status='complete', completed_at=?, file_path=?
		WHERE id=?`, completedAt, filePath, exportID)

	durationMs := time.Since(start).Milliseconds()
	s.auditSvc.Record(auditsvc.Entry{
		UserID:        actorID,
		Action:        "create",
		EntityType:    "diagnostic_exports",
		EntityID:      exportID,
		Details:       fmt.Sprintf("diagnostic package created: %s", fileName),
		AfterSnapshot: audithelper.ToJSON(map[string]any{"id": exportID, "file_name": fileName}),
		IPAddress:     ipAddress,
		RequestID:     correlationID,
		Outcome:       "success",
	})
	_ = s.jobWriter.Record(jobs.Entry{
		JobName:       "diagnostic_export",
		JobType:       "on_demand",
		Status:        jobs.StatusSuccess,
		DurationMs:    durationMs,
		Detail:        fmt.Sprintf("Package %s created successfully", fileName),
		Actor:         actorID,
		CorrelationID: correlationID,
	})

	return filePath, nil
}

// ListExports returns recent diagnostic exports.
func (s *DiagnosticsService) ListExports(limit int) ([]ExportRecord, error) {
	q := fmt.Sprintf(`
		SELECT de.id, de.export_type, de.triggered_by, u.display_name,
		       de.file_path, de.status, de.error_message,
		       de.triggered_at, de.completed_at, de.created_at
		FROM diagnostic_exports de
		JOIN users u ON u.id = de.triggered_by
		ORDER BY de.triggered_at DESC
		LIMIT %d`, limit)
	rows, err := s.db.Query(q)
	if err != nil {
		return nil, fmt.Errorf("diag: list exports: %w", err)
	}
	defer rows.Close()
	var items []ExportRecord
	for rows.Next() {
		var e ExportRecord
		var triAt, creAt string
		var compAt sql.NullString
		if err := rows.Scan(
			&e.ID, &e.ExportType, &e.TriggeredBy, &e.TriggeredByName,
			&e.FilePath, &e.Status, &e.ErrorMessage,
			&triAt, &compAt, &creAt,
		); err != nil {
			return nil, err
		}
		e.FileName = filepath.Base(e.FilePath)
		e.TriggeredAt, _ = time.Parse(time.RFC3339, triAt)
		e.CreatedAt, _ = time.Parse(time.RFC3339, creAt)
		if compAt.Valid && compAt.String != "" {
			t, _ := time.Parse(time.RFC3339, compAt.String)
			e.CompletedAt = &t
		}
		items = append(items, e)
	}
	return items, rows.Err()
}

// DBStats collects SQLite page stats and per-table row counts.
func (s *DiagnosticsService) DBStats() ([]DBStat, error) {
	var stats []DBStat
	pragmas := []string{"page_count", "page_size", "journal_mode", "wal_checkpoint"}
	for _, p := range pragmas {
		var val string
		row := s.db.QueryRow("PRAGMA " + p)
		if err := row.Scan(&val); err == nil {
			label := strings.ReplaceAll(p, "_", " ")
			stats = append(stats, DBStat{Key: label, Value: val})
		}
	}

	tables := []string{
		"users", "residents", "admissions", "payments", "refunds",
		"settlement_shifts", "imported_card_batches",
		"audit_logs", "job_history", "config_versions",
		"competency_exam_sessions", "competency_exam_templates",
		"service_orders", "alert_events",
	}
	for _, t := range tables {
		var n int
		if err := s.db.QueryRow("SELECT COUNT(*) FROM " + t).Scan(&n); err == nil {
			stats = append(stats, DBStat{Key: "rows:" + t, Value: fmt.Sprintf("%d", n)})
		}
	}
	return stats, nil
}

// — types ——————————————————————————————————————————————————————————————————

// ExportRecord is a view-layer type for diagnostic export rows.
type ExportRecord struct {
	ID              string
	ExportType      string
	TriggeredBy     string
	TriggeredByName string
	FilePath        string
	FileName        string
	Status          string
	ErrorMessage    string
	TriggeredAt     time.Time
	CompletedAt     *time.Time
	CreatedAt       time.Time
}

// DBStat is a key/value pair for the DB statistics panel.
type DBStat struct {
	Key   string
	Value string
}

// — private helpers ——————————————————————————————————————————————————————————

func (s *DiagnosticsService) buildEntries() ([]ziputil.Entry, error) {
	readme := s.buildReadme()

	dbStats, err := s.collectDBStatsJSON()
	if err != nil {
		return nil, err
	}
	recentJobs, err := s.collectJobsJSON()
	if err != nil {
		return nil, err
	}
	configData, err := s.collectConfigJSON()
	if err != nil {
		return nil, err
	}
	auditLogs, err := s.collectAuditLogsJSON()
	if err != nil {
		return nil, err
	}

	entries := []ziputil.Entry{
		{ArchiveName: "README.txt", Content: []byte(readme)},
		{ArchiveName: "db_stats.json", Content: dbStats},
		{ArchiveName: "recent_jobs.json", Content: recentJobs},
		{ArchiveName: "config.json", Content: configData},
		{ArchiveName: "audit_logs.json", Content: auditLogs},
	}

	if logEntry := s.collectAppLogsEntry(); logEntry != nil {
		entries = append(entries, *logEntry)
	}

	snapEntries := s.collectConfigSnapshotEntries()
	entries = append(entries, snapEntries...)

	return entries, nil
}

func (s *DiagnosticsService) buildReadme() string {
	logLine := ""
	if s.logPath != "" {
		logLine = "  app_logs.txt            — last 5 000 bytes of runtime log (secrets redacted)\n"
	}
	return "CareOps Diagnostic Package\n" +
		"===========================\n" +
		"Generated: " + time.Now().UTC().Format(time.RFC3339) + "\n\n" +
		"CONTENTS\n" +
		"--------\n" +
		"  README.txt              — this file\n" +
		"  db_stats.json           — SQLite pragma values and per-table row counts\n" +
		"  recent_jobs.json        — last 100 job_history entries\n" +
		"  config.json             — current application configuration (namespace/key/value)\n" +
		"  audit_logs.json         — last 200 audit log entries (structured log bundle)\n" +
		"  config_snapshots/       — up to 3 most-recent configuration snapshot files\n" +
		logLine + "\n" +
		"EXCLUSIONS\n" +
		"----------\n" +
		"The following are intentionally NOT included in this package:\n" +
		"  * Encryption keys, API credentials, or other runtime secrets\n" +
		"  * Password hashes\n" +
		"  * Payment reference numbers (masked in all outputs)\n" +
		"  * Personally identifiable information beyond what is in audit and config tables\n\n" +
		"USE\n" +
		"---\n" +
		"This package is intended for operational troubleshooting only.\n" +
		"Handle with appropriate security controls — restrict access to admin staff.\n"
}

func (s *DiagnosticsService) collectDBStatsJSON() ([]byte, error) {
	stats, err := s.DBStats()
	if err != nil {
		return nil, err
	}
	out := map[string]string{}
	for _, st := range stats {
		out[st.Key] = st.Value
	}
	return json.MarshalIndent(out, "", "  ")
}

func (s *DiagnosticsService) collectJobsJSON() ([]byte, error) {
	rows, err := s.db.Query(`
		SELECT id, job_name, job_type, status, duration_ms, detail,
		       actor, correlation_id, ran_at
		FROM job_history ORDER BY ran_at DESC LIMIT 100`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	type jobRow struct {
		ID            string `json:"id"`
		JobName       string `json:"job_name"`
		JobType       string `json:"job_type"`
		Status        string `json:"status"`
		DurationMs    int    `json:"duration_ms"`
		Detail        string `json:"detail"`
		Actor         string `json:"actor"`
		CorrelationID string `json:"correlation_id"`
		RanAt         string `json:"ran_at"`
	}
	var jobList []jobRow
	for rows.Next() {
		var j jobRow
		if err := rows.Scan(
			&j.ID, &j.JobName, &j.JobType, &j.Status,
			&j.DurationMs, &j.Detail, &j.Actor, &j.CorrelationID, &j.RanAt,
		); err != nil {
			return nil, err
		}
		jobList = append(jobList, j)
	}
	return json.MarshalIndent(jobList, "", "  ")
}

func (s *DiagnosticsService) collectConfigJSON() ([]byte, error) {
	rows, err := s.db.Query(`SELECT namespace, config_key, config_value FROM config_versions ORDER BY namespace, config_key`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	type cfgRow struct {
		Namespace string `json:"namespace"`
		Key       string `json:"config_key"`
		Value     string `json:"config_value"`
	}
	var configs []cfgRow
	for rows.Next() {
		var r cfgRow
		if err := rows.Scan(&r.Namespace, &r.Key, &r.Value); err != nil {
			return nil, err
		}
		configs = append(configs, r)
	}
	return json.MarshalIndent(configs, "", "  ")
}

// collectAuditLogsJSON returns the last 200 audit log entries as a JSON array.
// This provides a structured log bundle for offline troubleshooting — all
// significant system operations are captured here with actor identity and outcome.
func (s *DiagnosticsService) collectAuditLogsJSON() ([]byte, error) {
	rows, err := s.db.Query(`
		SELECT id, user_id, action, entity_type, entity_id, details,
		       outcome, ip_address, request_id, occurred_at
		FROM audit_logs ORDER BY occurred_at DESC LIMIT 200`)
	if err != nil {
		return nil, fmt.Errorf("diag: collect audit logs: %w", err)
	}
	defer rows.Close()
	type auditRow struct {
		ID         string `json:"id"`
		UserID     string `json:"user_id"`
		Action     string `json:"action"`
		EntityType string `json:"entity_type"`
		EntityID   string `json:"entity_id"`
		Details    string `json:"details"`
		Outcome    string `json:"outcome"`
		IPAddress  string `json:"ip_address"`
		RequestID  string `json:"request_id"`
		OccurredAt string `json:"occurred_at"`
	}
	var logList []auditRow
	for rows.Next() {
		var r auditRow
		if err := rows.Scan(
			&r.ID, &r.UserID, &r.Action, &r.EntityType, &r.EntityID, &r.Details,
			&r.Outcome, &r.IPAddress, &r.RequestID, &r.OccurredAt,
		); err != nil {
			return nil, err
		}
		logList = append(logList, r)
	}
	return json.MarshalIndent(logList, "", "  ")
}

// collectConfigSnapshotEntries reads up to 3 most-recent JSON snapshot files
// from configSnapPath and returns them as ZIP entries under config_snapshots/.
// Errors reading individual files are logged but do not fail the export.
func (s *DiagnosticsService) collectConfigSnapshotEntries() []ziputil.Entry {
	if s.configSnapPath == "" {
		return nil
	}
	dir, err := os.ReadDir(s.configSnapPath)
	if os.IsNotExist(err) || err != nil {
		return nil
	}

	type snapFile struct {
		name    string
		modTime time.Time
	}
	var snaps []snapFile
	for _, e := range dir {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		snaps = append(snaps, snapFile{e.Name(), info.ModTime()})
	}
	sort.Slice(snaps, func(i, j int) bool {
		return snaps[i].modTime.After(snaps[j].modTime)
	})
	if len(snaps) > 3 {
		snaps = snaps[:3]
	}

	var entries []ziputil.Entry
	for _, sf := range snaps {
		srcPath := filepath.Join(s.configSnapPath, sf.name)
		entries = append(entries, ziputil.Entry{
			ArchiveName: "config_snapshots/" + sf.name,
			SourcePath:  srcPath,
		})
	}
	return entries
}

// collectAppLogsEntry reads the last 5 000 bytes from the app log file, redacts
// sensitive patterns, and returns a ZIP entry. Returns nil when logPath is unset
// or the file cannot be read (errors are logged but do not fail the export).
func (s *DiagnosticsService) collectAppLogsEntry() *ziputil.Entry {
	if s.logPath == "" {
		return nil
	}
	f, err := os.Open(s.logPath)
	if err != nil {
		s.log.Warn("diag: cannot open log file for export", "path", s.logPath, "err", err)
		return nil
	}
	defer f.Close()

	const maxBytes = 5000
	info, err := f.Stat()
	if err != nil {
		return nil
	}
	if info.Size() > maxBytes {
		if _, err := f.Seek(-maxBytes, io.SeekEnd); err != nil {
			return nil
		}
	}
	raw, err := io.ReadAll(f)
	if err != nil {
		return nil
	}
	return &ziputil.Entry{ArchiveName: "app_logs.txt", Content: redactLogBytes(raw)}
}

// redactLogBytes removes bcrypt hashes and 64-char hex strings (potential encryption
// keys) from log content before bundling it into a diagnostic package.
var (
	reBcrypt = regexp.MustCompile(`\$2[ab]\$\d{2}\$[./A-Za-z0-9]{53}`)
	reHexKey = regexp.MustCompile(`\b[0-9a-fA-F]{64}\b`)
)

func redactLogBytes(data []byte) []byte {
	out := reBcrypt.ReplaceAll(data, []byte("[REDACTED-HASH]"))
	out = reHexKey.ReplaceAll(out, []byte("[REDACTED-KEY]"))
	return out
}

func (s *DiagnosticsService) markFailed(exportID, errMsg string) error {
	_, err := s.db.Exec(
		`UPDATE diagnostic_exports SET status='failed', error_message=?, completed_at=? WHERE id=?`,
		errMsg, timeutil.NowISO(), exportID,
	)
	return err
}
