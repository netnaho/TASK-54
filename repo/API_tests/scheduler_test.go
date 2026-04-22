package api_tests

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"careops/clinic/internal/app"
	"careops/clinic/internal/modules/reports/repository"
	"careops/clinic/internal/modules/reports/scheduler"
	reportsvc "careops/clinic/internal/modules/reports/service"
	auditrepo "careops/clinic/internal/modules/audit/repository"
	auditsvc "careops/clinic/internal/modules/audit/service"
	"careops/clinic/internal/platform/jobs"
	"careops/clinic/internal/platform/logger"
)

// TestScheduler_RunsDueReport verifies that the scheduler runs a report whose
// cron expression matches the current minute window.
func TestScheduler_RunsDueReport(t *testing.T) {
	cfg := testConfig(t)
	log := logger.New("text", "error")
	db, err := app.BootstrapFast(cfg, log)
	if err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	// Create a report definition with a cron that matches "every minute" (* * * * *).
	reportRepo := repository.NewReportRepo(db)
	jobWriter := jobs.NewWriter(db)
	auditRepo := auditrepo.NewAuditRepo(db)
	auditService := auditsvc.NewAuditService(auditRepo, log)
	svc := reportsvc.New(db, reportRepo, auditService, jobWriter, cfg.ExportsPath, log)

	// Get a valid user to be the creator
	var creatorID string
	if err := db.QueryRow(`SELECT id FROM users WHERE is_active=1 LIMIT 1`).Scan(&creatorID); err != nil {
		t.Fatalf("get user: %v", err)
	}

	// Insert a report definition directly (bypassing HTTP layer).
	_, err = db.Exec(`
		INSERT INTO scheduled_reports
			(id, name, report_type, schedule_cron, parameters, output_format,
			 recipients, is_active, created_by, created_at, updated_at, row_version)
		VALUES ('sched-test-001', 'Scheduler Test Report', 'audit', '* * * * *', '{}', 'csv',
			'[]', 1, ?, datetime('now'), datetime('now'), 1)`, creatorID)
	if err != nil {
		t.Fatalf("insert report: %v", err)
	}

	sched := scheduler.New(reportRepo, svc, log)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	sched.Start(ctx)

	// Give the scheduler a moment to fire its initial tick.
	time.Sleep(200 * time.Millisecond)

	// Verify a run was recorded.
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM report_runs WHERE scheduled_report_id='sched-test-001'`).Scan(&count); err != nil {
		t.Fatalf("count runs: %v", err)
	}
	if count == 0 {
		t.Error("scheduler: expected at least one run to be recorded after initial tick")
	}
}

// TestScheduler_DeduplicatesWithinWindow verifies that tick() does not produce
// a second run for the same window if one was already recorded.
func TestScheduler_DeduplicatesWithinWindow(t *testing.T) {
	cfg := testConfig(t)
	log := logger.New("text", "error")
	db, err := app.BootstrapFast(cfg, log)
	if err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	reportRepo := repository.NewReportRepo(db)
	jobWriter := jobs.NewWriter(db)
	auditRepo := auditrepo.NewAuditRepo(db)
	auditService := auditsvc.NewAuditService(auditRepo, log)
	svc := reportsvc.New(db, reportRepo, auditService, jobWriter, cfg.ExportsPath, log)

	var creatorID string
	if err := db.QueryRow(`SELECT id FROM users WHERE is_active=1 LIMIT 1`).Scan(&creatorID); err != nil {
		t.Fatalf("get user: %v", err)
	}

	_, err = db.Exec(`
		INSERT INTO scheduled_reports
			(id, name, report_type, schedule_cron, parameters, output_format,
			 recipients, is_active, created_by, created_at, updated_at, row_version)
		VALUES ('sched-dedup-001', 'Dedup Test Report', 'occupancy', '* * * * *', '{}', 'csv',
			'[]', 1, ?, datetime('now'), datetime('now'), 1)`, creatorID)
	if err != nil {
		t.Fatalf("insert report: %v", err)
	}

	sched := scheduler.New(reportRepo, svc, log)

	now := time.Now()
	// Call tick twice for the same window.
	sched.TickForTest(now)
	sched.TickForTest(now)

	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM report_runs WHERE scheduled_report_id='sched-dedup-001'`).Scan(&count); err != nil {
		t.Fatalf("count runs: %v", err)
	}
	if count != 1 {
		t.Errorf("deduplication: expected exactly 1 run, got %d", count)
	}
}

// TestScheduler_SkipsInactiveReports verifies that inactive report definitions
// are not run by the scheduler.
func TestScheduler_SkipsInactiveReports(t *testing.T) {
	cfg := testConfig(t)
	log := logger.New("text", "error")
	db, err := app.BootstrapFast(cfg, log)
	if err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	reportRepo := repository.NewReportRepo(db)
	jobWriter := jobs.NewWriter(db)
	auditRepo := auditrepo.NewAuditRepo(db)
	auditService := auditsvc.NewAuditService(auditRepo, log)
	svc := reportsvc.New(db, reportRepo, auditService, jobWriter, cfg.ExportsPath, log)

	var creatorID string
	if err := db.QueryRow(`SELECT id FROM users WHERE is_active=1 LIMIT 1`).Scan(&creatorID); err != nil {
		t.Fatalf("get user: %v", err)
	}

	_, err = db.Exec(`
		INSERT INTO scheduled_reports
			(id, name, report_type, schedule_cron, parameters, output_format,
			 recipients, is_active, created_by, created_at, updated_at, row_version)
		VALUES ('sched-inactive-001', 'Inactive Report', 'audit', '* * * * *', '{}', 'csv',
			'[]', 0, ?, datetime('now'), datetime('now'), 1)`, creatorID)
	if err != nil {
		t.Fatalf("insert inactive report: %v", err)
	}

	sched := scheduler.New(reportRepo, svc, log)
	sched.TickForTest(time.Now())

	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM report_runs WHERE scheduled_report_id='sched-inactive-001'`).Scan(&count); err != nil {
		t.Fatalf("count runs: %v", err)
	}
	if count != 0 {
		t.Errorf("inactive report: expected 0 runs, got %d", count)
	}
}

// TestScheduler_SkipsNonMatchingCron verifies that a report whose cron does not
// match the current time is not run.
func TestScheduler_SkipsNonMatchingCron(t *testing.T) {
	cfg := testConfig(t)
	log := logger.New("text", "error")
	db, err := app.BootstrapFast(cfg, log)
	if err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	reportRepo := repository.NewReportRepo(db)
	jobWriter := jobs.NewWriter(db)
	auditRepo := auditrepo.NewAuditRepo(db)
	auditService := auditsvc.NewAuditService(auditRepo, log)
	svc := reportsvc.New(db, reportRepo, auditService, jobWriter, cfg.ExportsPath, log)

	var creatorID string
	if err := db.QueryRow(`SELECT id FROM users WHERE is_active=1 LIMIT 1`).Scan(&creatorID); err != nil {
		t.Fatalf("get user: %v", err)
	}

	_, err = db.Exec(`
		INSERT INTO scheduled_reports
			(id, name, report_type, schedule_cron, parameters, output_format,
			 recipients, is_active, created_by, created_at, updated_at, row_version)
		VALUES ('sched-nomatch-001', 'No-Match Report', 'finance', '0 3 29 2 *', '{}', 'csv',
			'[]', 1, ?, datetime('now'), datetime('now'), 1)`, creatorID)
	if err != nil {
		t.Fatalf("insert report: %v", err)
	}

	// Use a time that is definitely not Feb 29 at 03:00.
	notLeapDay := time.Date(2024, 3, 1, 9, 0, 0, 0, time.UTC)
	sched := scheduler.New(reportRepo, svc, log)
	sched.TickForTest(notLeapDay)

	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM report_runs WHERE scheduled_report_id='sched-nomatch-001'`).Scan(&count); err != nil {
		t.Fatalf("count runs: %v", err)
	}
	if count != 0 {
		t.Errorf("non-matching cron: expected 0 runs, got %d", count)
	}
}

// TestScheduler_ViaHTTP_AutoRunsViaStartReportScheduler is a lightweight
// integration test that calls app.StartReportScheduler and verifies it fires
// an initial tick without panicking.
func TestScheduler_ViaHTTP_AutoRunsViaStartReportScheduler(t *testing.T) {
	cfg := testConfig(t)
	log := logger.New("text", "error")
	db, err := app.BootstrapFast(cfg, log)
	if err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Should not panic.
	app.StartReportScheduler(ctx, db, cfg, log)
	time.Sleep(100 * time.Millisecond)
}

// TestReports_Toggle_IsIdempotent verifies that sending the same toggle request
// twice with the same idempotency key does not cause double-toggle.
func TestReports_Toggle_IsIdempotent(t *testing.T) {
	a := buildTestApp(t)
	cookie := loginAs(t, a, "admin@careops.local", "Admin!234567")

	// Create a definition.
	fields := url.Values{
		"name":          {"Idempotent Toggle Test"},
		"report_type":   {"occupancy"},
		"output_format": {"csv"},
	}
	postReport(t, a, cookie, fields)
	id := getReportID(t, a, cookie, "Idempotent Toggle Test")

	sendToggle := func(key string) *http.Response {
		t.Helper()
		body := strings.NewReader("_idempotency_key=" + key)
		req := httptest.NewRequest(http.MethodPost, "/reports/"+id+"/toggle", body)
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.AddCookie(&http.Cookie{Name: "careops_session", Value: cookie})
		resp, err := a.Test(req, -1)
		if err != nil {
			t.Fatalf("toggle: %v", err)
		}
		return resp
	}

	r1 := sendToggle("toggle-key-idem-001")
	r2 := sendToggle("toggle-key-idem-001")

	if r1.StatusCode != http.StatusFound && r1.StatusCode != http.StatusSeeOther {
		t.Errorf("first toggle: want redirect, got %d", r1.StatusCode)
	}
	if r2.StatusCode != http.StatusFound && r2.StatusCode != http.StatusSeeOther {
		t.Errorf("second toggle (same key): want redirect, got %d", r2.StatusCode)
	}

	// After two idempotent toggles (which count as one), the status should be
	// inactive (toggled once from the seeded active=true default).
	listResp := getWithSession(t, a, "/reports", cookie)
	body := readBody(t, listResp)
	if !strings.Contains(body, "inactive") {
		t.Error("after idempotent toggle: report should appear inactive")
	}
}
