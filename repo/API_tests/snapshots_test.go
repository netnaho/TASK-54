package api_tests

// Issue 3 — Uniform before/after snapshots
//
// These tests assert that privileged mutations write non-empty, structurally
// valid JSON into the before_snapshot and/or after_snapshot columns of
// audit_logs. Tests use the service layer directly so the exact DB state can
// be inspected without going through the HTML renderer.

import (
	"database/sql"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"careops/clinic/internal/app"
	"careops/clinic/internal/config"
	auditrepo "careops/clinic/internal/modules/audit/repository"
	auditsvc "careops/clinic/internal/modules/audit/service"
	cfgvrepo "careops/clinic/internal/modules/config_versions/repository"
	cfgvsvc "careops/clinic/internal/modules/config_versions/service"
	diagsvc "careops/clinic/internal/modules/diagnostics/service"
	examdomain "careops/clinic/internal/modules/exams/domain"
	examrepo "careops/clinic/internal/modules/exams/repository"
	examsvc "careops/clinic/internal/modules/exams/service"
	financedomain "careops/clinic/internal/modules/finance/domain"
	financerepo "careops/clinic/internal/modules/finance/repository"
	financesvc "careops/clinic/internal/modules/finance/service"
	"careops/clinic/internal/platform/idempotency"
	"careops/clinic/internal/platform/jobs"
	"careops/clinic/internal/platform/logger"
	reportsdomain "careops/clinic/internal/modules/reports/domain"
	reportsrepo "careops/clinic/internal/modules/reports/repository"
	reportssvc "careops/clinic/internal/modules/reports/service"
	"careops/clinic/internal/shared/idgen"
)

// assertValidJSON fails the test if s is empty or not valid JSON.
func assertValidJSON(t *testing.T, label, s string) {
	t.Helper()
	if s == "" {
		t.Errorf("%s: snapshot must not be empty", label)
		return
	}
	var v any
	if err := json.Unmarshal([]byte(s), &v); err != nil {
		t.Errorf("%s: snapshot is not valid JSON (%q): %v", label, s[:min(80, len(s))], err)
	}
}

// queryLatestAuditSnapshot returns (before_snapshot, after_snapshot) for the
// most recent audit entry matching action+entity_type.
func queryLatestAuditSnapshot(t *testing.T, db *sql.DB, action, entityType string) (string, string) {
	t.Helper()
	var before, after string
	err := db.QueryRow(`
		SELECT COALESCE(before_snapshot,''), COALESCE(after_snapshot,'')
		FROM audit_logs
		WHERE action = ? AND entity_type = ?
		ORDER BY occurred_at DESC LIMIT 1`, action, entityType,
	).Scan(&before, &after)
	if err == sql.ErrNoRows {
		t.Fatalf("no audit entry found for action=%q entity_type=%q", action, entityType)
	}
	if err != nil {
		t.Fatalf("query audit snapshot: %v", err)
	}
	return before, after
}

// newFinanceSvc returns a wired FinanceService for snapshot tests.
func newFinanceSvc(t *testing.T, db *sql.DB, log *slog.Logger) *financesvc.FinanceService {
	t.Helper()
	cfg := testConfig(t)
	ar := auditrepo.NewAuditRepo(db)
	as := auditsvc.NewAuditService(ar, log)
	idem := idempotency.NewService(db)
	jw := jobs.NewWriter(db)
	return financesvc.New(
		financerepo.NewPaymentRepo(db),
		financerepo.NewRefundRepo(db),
		financerepo.NewShiftRepo(db),
		financerepo.NewBatchRepo(db),
		as, idem, nil, false, cfg.ExportsPath, jw, log,
	)
}

// newExamSched returns a wired ExamScheduler for snapshot tests.
func newExamSched(t *testing.T, db *sql.DB, log *slog.Logger) *examsvc.ExamScheduler {
	t.Helper()
	ar := auditrepo.NewAuditRepo(db)
	as := auditsvc.NewAuditService(ar, log)
	idem := idempotency.NewService(db)
	return examsvc.NewExamScheduler(
		examrepo.NewTemplateRepo(db),
		examrepo.NewSessionRepo(db),
		examrepo.NewConflictRepo(db),
		as, idem, time.UTC, log,
	)
}

// newReportSvc returns a wired ReportService for snapshot tests.
func newReportSvc(t *testing.T, db *sql.DB, log *slog.Logger) *reportssvc.ReportService {
	t.Helper()
	cfg := testConfig(t)
	ar := auditrepo.NewAuditRepo(db)
	as := auditsvc.NewAuditService(ar, log)
	jw := jobs.NewWriter(db)
	return reportssvc.New(db, reportsrepo.NewReportRepo(db), as, jw, cfg.ExportsPath, log)
}

// newDiagSvc returns a wired DiagnosticsService for snapshot tests.
func newDiagSvc(t *testing.T, cfg *config.Config, db *sql.DB, log *slog.Logger) *diagsvc.DiagnosticsService {
	t.Helper()
	ar := auditrepo.NewAuditRepo(db)
	as := auditsvc.NewAuditService(ar, log)
	jw := jobs.NewWriter(db)
	return diagsvc.New(db, as, jw, cfg.DiagnosticsPath, cfg.ConfigSnapPath, log)
}

// newCfgSvc returns a wired ConfigService for snapshot tests.
func newCfgSvc(t *testing.T, cfg *config.Config, db *sql.DB, log *slog.Logger) *cfgvsvc.ConfigService {
	t.Helper()
	ar := auditrepo.NewAuditRepo(db)
	as := auditsvc.NewAuditService(ar, log)
	jw := jobs.NewWriter(db)
	return cfgvsvc.New(cfgvrepo.NewConfigRepo(db), as, jw, cfg.ConfigSnapPath, log)
}

// ── Finance — shift close ────────────────────────────────────────────────────

// TestSnapshot_Finance_CloseShift_HasBeforeAndAfter verifies that closing a
// shift writes non-empty before_snapshot and after_snapshot.
func TestSnapshot_Finance_CloseShift_HasBeforeAndAfter(t *testing.T) {
	cfg := testConfig(t)
	log := logger.New("text", "error")
	db, err := app.BootstrapFast(cfg, log)
	if err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	svc := newFinanceSvc(t, db, log)

	var actorID string
	if err := db.QueryRow(`SELECT id FROM users WHERE is_active=1 LIMIT 1`).Scan(&actorID); err != nil {
		t.Fatalf("get user: %v", err)
	}

	shift, err := svc.GetOrOpenShift("2026-05-01", "morning", actorID, "", "", "")
	if err != nil {
		t.Fatalf("open shift: %v", err)
	}

	_, err = svc.CloseShift(financedomain.CloseShiftCmd{ShiftID: shift.ID}, actorID, "", "", "")
	if err != nil {
		t.Fatalf("close shift: %v", err)
	}

	before, after := queryLatestAuditSnapshot(t, db, "update", "settlement_shifts")
	assertValidJSON(t, "CloseShift before_snapshot", before)
	assertValidJSON(t, "CloseShift after_snapshot", after)

	// Before snapshot must reflect an open shift.
	var beforeMap map[string]any
	json.Unmarshal([]byte(before), &beforeMap) //nolint:errcheck
	statusVal := firstOf(beforeMap, "Status", "status")
	if statusVal != "open" {
		t.Errorf("CloseShift before_snapshot: expected status=open, got %v", statusVal)
	}
}

// TestSnapshot_Finance_GetOrOpenShift_HasAfterSnapshot verifies that opening a
// new shift writes a non-empty after_snapshot.
func TestSnapshot_Finance_GetOrOpenShift_HasAfterSnapshot(t *testing.T) {
	cfg := testConfig(t)
	log := logger.New("text", "error")
	db, err := app.BootstrapFast(cfg, log)
	if err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	svc := newFinanceSvc(t, db, log)

	var actorID string
	db.QueryRow(`SELECT id FROM users WHERE is_active=1 LIMIT 1`).Scan(&actorID) //nolint:errcheck

	_, err = svc.GetOrOpenShift("2026-06-01", "afternoon", actorID, "", "", "")
	if err != nil {
		t.Fatalf("open shift: %v", err)
	}

	_, after := queryLatestAuditSnapshot(t, db, "create", "settlement_shifts")
	assertValidJSON(t, "GetOrOpenShift after_snapshot", after)
}

// TestSnapshot_Finance_ProcessRefund_HasBeforeAndAfter verifies that processing
// a refund writes before_snapshot (payment state) and after_snapshot (refund).
func TestSnapshot_Finance_ProcessRefund_HasBeforeAndAfter(t *testing.T) {
	cfg := testConfig(t)
	log := logger.New("text", "error")
	db, err := app.BootstrapFast(cfg, log)
	if err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	svc := newFinanceSvc(t, db, log)

	var actorID, residentID string
	db.QueryRow(`SELECT id FROM users WHERE is_active=1 LIMIT 1`).Scan(&actorID) //nolint:errcheck
	if err := db.QueryRow(`SELECT id FROM residents LIMIT 1`).Scan(&residentID); err != nil {
		t.Skip("no residents seeded")
	}

	shift, _ := svc.GetOrOpenShift("2026-05-02", "night", actorID, "", "", "")
	p, err := svc.RecordPayment(financedomain.CreatePayment{
		ResidentID:      residentID,
		ShiftID:         shift.ID,
		PaymentMethod:   financedomain.MethodCash,
		AmountDollarsIn: "40.00",
		Description:     "refund snapshot test",
		Currency:        "USD",
	}, actorID, "", "", "")
	if err != nil {
		t.Fatalf("record payment: %v", err)
	}

	_, err = svc.ProcessRefund(financedomain.CreateRefund{
		PaymentID:       p.ID,
		AmountDollarsIn: "10.00",
		Reason:          "snapshot test refund",
	}, actorID, "", "", "")
	if err != nil {
		t.Fatalf("process refund: %v", err)
	}

	before, after := queryLatestAuditSnapshot(t, db, "create", "refunds")
	assertValidJSON(t, "ProcessRefund before_snapshot", before)
	assertValidJSON(t, "ProcessRefund after_snapshot", after)

	// before_snapshot must reference the original payment.
	var bm map[string]any
	json.Unmarshal([]byte(before), &bm) //nolint:errcheck
	paymentIDVal := firstOf(bm, "payment_id", "PaymentID", "id", "ID")
	if paymentIDVal != p.ID {
		t.Errorf("ProcessRefund before_snapshot: expected payment_id=%q, got %v", p.ID, paymentIDVal)
	}
}

// TestSnapshot_Finance_ImportCardBatch_HasAfterSnapshot verifies that a batch
// import writes a non-empty after_snapshot describing batch statistics.
func TestSnapshot_Finance_ImportCardBatch_HasAfterSnapshot(t *testing.T) {
	cfg := testConfig(t)
	log := logger.New("text", "error")
	db, err := app.BootstrapFast(cfg, log)
	if err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	svc := newFinanceSvc(t, db, log)

	var actorID string
	db.QueryRow(`SELECT id FROM users WHERE is_active=1 LIMIT 1`).Scan(&actorID) //nolint:errcheck

	csvContent := "resident_mrn,amount_dollars,description\nMRN-00001,5.00,snapshot batch test\n"
	shift, _ := svc.GetOrOpenShift("2026-05-03", "morning", actorID, "", "", "")
	_, err = svc.ImportCardBatch("snap_test.csv",
		strings.NewReader(csvContent),
		actorID, shift.ID, "", "", "")
	if err != nil {
		t.Fatalf("ImportCardBatch: %v", err)
	}

	_, after := queryLatestAuditSnapshot(t, db, "create", "imported_card_batches")
	assertValidJSON(t, "ImportCardBatch after_snapshot", after)

	var m map[string]any
	json.Unmarshal([]byte(after), &m) //nolint:errcheck
	if firstOf(m, "batch_id", "BatchID") == nil {
		t.Error("ImportCardBatch after_snapshot: expected batch_id key")
	}
}

// ── Exam scheduler mutations ─────────────────────────────────────────────────

// TestSnapshot_Exam_CreateTemplate_HasAfterSnapshot verifies that creating an
// exam template writes a non-empty after_snapshot.
func TestSnapshot_Exam_CreateTemplate_HasAfterSnapshot(t *testing.T) {
	cfg := testConfig(t)
	log := logger.New("text", "error")
	db, err := app.BootstrapFast(cfg, log)
	if err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	sched := newExamSched(t, db, log)

	var creatorID string
	db.QueryRow(`SELECT id FROM users WHERE is_active=1 LIMIT 1`).Scan(&creatorID) //nolint:errcheck

	_, err = sched.CreateTemplate(examdomain.CreateTemplate{
		Title:           "Snapshot Create Test",
		Description:     "snapshot test template",
		DurationMinutes: 60,
		PassingScore:    70,
		RoleScope:       "nurse",
		WindowStart:     "09:00",
		WindowEnd:       "17:00",
	}, creatorID, "", "")
	if err != nil {
		t.Fatalf("CreateTemplate: %v", err)
	}

	_, after := queryLatestAuditSnapshot(t, db, "exam_template.create", "competency_exam_templates")
	assertValidJSON(t, "CreateTemplate after_snapshot", after)
}

// TestSnapshot_Exam_UpdateTemplate_HasBeforeAndAfter verifies that updating an
// exam template writes both before_snapshot and after_snapshot.
func TestSnapshot_Exam_UpdateTemplate_HasBeforeAndAfter(t *testing.T) {
	cfg := testConfig(t)
	log := logger.New("text", "error")
	db, err := app.BootstrapFast(cfg, log)
	if err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	sched := newExamSched(t, db, log)

	var creatorID string
	db.QueryRow(`SELECT id FROM users WHERE is_active=1 LIMIT 1`).Scan(&creatorID) //nolint:errcheck

	tmpl, err := sched.CreateTemplate(examdomain.CreateTemplate{
		Title:           "Before Update Template",
		Description:     "will be updated",
		DurationMinutes: 30,
		PassingScore:    70,
		RoleScope:       "nurse",
		WindowStart:     "08:00",
		WindowEnd:       "16:00",
	}, creatorID, "", "")
	if err != nil {
		t.Fatalf("CreateTemplate: %v", err)
	}

	_, err = sched.UpdateTemplate(tmpl.ID, examdomain.UpdateTemplate{
		Title:           "After Update Template",
		Description:     "was updated",
		DurationMinutes: 45,
		PassingScore:    70,
		WindowStart:     "08:00",
		WindowEnd:       "16:00",
	}, tmpl.TemplateVersion, creatorID, "", "")
	if err != nil {
		t.Fatalf("UpdateTemplate: %v", err)
	}

	before, after := queryLatestAuditSnapshot(t, db, "exam_template.update", "competency_exam_templates")
	assertValidJSON(t, "UpdateTemplate before_snapshot", before)
	assertValidJSON(t, "UpdateTemplate after_snapshot", after)
}

// TestSnapshot_Exam_PublishSession_HasBeforeAndAfter verifies that publishing
// an exam session writes before_snapshot (draft state) and after_snapshot.
func TestSnapshot_Exam_PublishSession_HasBeforeAndAfter(t *testing.T) {
	cfg := testConfig(t)
	log := logger.New("text", "error")
	db, err := app.BootstrapFast(cfg, log)
	if err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	sched := newExamSched(t, db, log)

	var actorID string
	db.QueryRow(`SELECT id FROM users WHERE is_active=1 LIMIT 1`).Scan(&actorID) //nolint:errcheck

	tmpl, err := sched.CreateTemplate(examdomain.CreateTemplate{
		Title: "Publish Snapshot Template", DurationMinutes: 60,
		PassingScore: 70, RoleScope: "nurse", WindowStart: "08:00", WindowEnd: "17:00",
	}, actorID, "", "")
	if err != nil {
		t.Fatalf("CreateTemplate: %v", err)
	}

	// Use 10:00 UTC two days out to stay inside the 08:00–17:00 window.
	start := time.Now().UTC().Truncate(24 * time.Hour).Add(48*time.Hour + 10*time.Hour)
	sess, _, err := sched.GenerateSession(examdomain.GenerateSessionCmd{
		TemplateID:   tmpl.ID,
		PlannedStart: start,
		Room:         "publish-snap-room",
		ProctorID:    "",
	}, actorID, "", "")
	if err != nil {
		t.Fatalf("GenerateSession: %v", err)
	}

	if err := sched.PublishSession(sess.ID, actorID, sess.RowVersion, "", "", ""); err != nil {
		t.Fatalf("PublishSession: %v", err)
	}

	before, after := queryLatestAuditSnapshot(t, db, "exam_session.publish", "competency_exam_sessions")
	assertValidJSON(t, "PublishSession before_snapshot", before)
	assertValidJSON(t, "PublishSession after_snapshot", after)

	// before must indicate IsDraft=true.
	var bm map[string]any
	json.Unmarshal([]byte(before), &bm) //nolint:errcheck
	if v := firstOf(bm, "IsDraft", "is_draft"); v != true {
		t.Errorf("PublishSession before_snapshot: expected IsDraft=true, got %v (full: %v)", v, bm)
	}
}

// TestSnapshot_Exam_ResolveConflict_HasBeforeAndAfter verifies that resolving a
// conflict writes before_snapshot (unresolved) and after_snapshot (resolved).
func TestSnapshot_Exam_ResolveConflict_HasBeforeAndAfter(t *testing.T) {
	cfg := testConfig(t)
	log := logger.New("text", "error")
	db, err := app.BootstrapFast(cfg, log)
	if err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	sched := newExamSched(t, db, log)

	var actorID string
	db.QueryRow(`SELECT id FROM users WHERE is_active=1 LIMIT 1`).Scan(&actorID) //nolint:errcheck

	tmpl, _ := sched.CreateTemplate(examdomain.CreateTemplate{
		Title: "Conflict Resolve Snapshot", DurationMinutes: 120,
		PassingScore: 70, RoleScope: "nurse", WindowStart: "08:00", WindowEnd: "18:00",
	}, actorID, "", "")

	// 10:00 UTC three days out — inside the 08:00–18:00 window.
	start := time.Now().UTC().Truncate(24 * time.Hour).Add(72*time.Hour + 10*time.Hour)
	// First session establishes the room/proctor slot.
	sched.GenerateSession(examdomain.GenerateSessionCmd{ //nolint:errcheck
		TemplateID: tmpl.ID, PlannedStart: start, Room: "conflict-snap-room", ProctorID: "",
	}, actorID, "", "")
	// Second session overlaps: conflict is detected and stored FOR sess2.
	sess2, _, _ := sched.GenerateSession(examdomain.GenerateSessionCmd{
		TemplateID: tmpl.ID, PlannedStart: start, Room: "conflict-snap-room", ProctorID: "",
	}, actorID, "", "")

	if sess2 == nil {
		t.Skip("GenerateSession returned nil — skipping resolve snapshot test")
	}
	var conflictID string
	db.QueryRow(
		`SELECT id FROM exam_scheduling_conflicts WHERE session_id=? AND resolved=0 LIMIT 1`,
		sess2.ID,
	).Scan(&conflictID)
	if conflictID == "" {
		t.Skip("no conflict generated — skipping resolve snapshot test")
	}

	if err := sched.ResolveConflict(conflictID, "snapshot test resolution", actorID, "", ""); err != nil {
		t.Fatalf("ResolveConflict: %v", err)
	}

	before, after := queryLatestAuditSnapshot(t, db, "exam_conflict.resolve", "exam_scheduling_conflicts")
	assertValidJSON(t, "ResolveConflict before_snapshot", before)
	assertValidJSON(t, "ResolveConflict after_snapshot", after)
}

// TestSnapshot_Exam_UpdateSessionBlock_HasAfterSnapshot verifies that moving a
// session's time block writes a non-empty after_snapshot.
func TestSnapshot_Exam_UpdateSessionBlock_HasAfterSnapshot(t *testing.T) {
	cfg := testConfig(t)
	log := logger.New("text", "error")
	db, err := app.BootstrapFast(cfg, log)
	if err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	sched := newExamSched(t, db, log)

	var actorID string
	db.QueryRow(`SELECT id FROM users WHERE is_active=1 LIMIT 1`).Scan(&actorID) //nolint:errcheck

	tmpl, _ := sched.CreateTemplate(examdomain.CreateTemplate{
		Title: "Block Update Snapshot", DurationMinutes: 60,
		PassingScore: 70, RoleScope: "nurse", WindowStart: "07:00", WindowEnd: "20:00",
	}, actorID, "", "")

	// 10:00 UTC four days out — inside the 07:00–20:00 window.
	start := time.Now().UTC().Truncate(24 * time.Hour).Add(96*time.Hour + 10*time.Hour)
	sess, _, _ := sched.GenerateSession(examdomain.GenerateSessionCmd{
		TemplateID: tmpl.ID, PlannedStart: start, Room: "block-snap-room", ProctorID: "",
	}, actorID, "", "")

	_, _, err = sched.UpdateSessionBlock(examdomain.UpdateSessionBlockCmd{
		SessionID:    sess.ID,
		PlannedStart: start.Add(time.Hour),
		Room:         "block-snap-room-2",
		ProctorID:    "",
		RowVersion:   sess.RowVersion,
	}, actorID, "", "")
	if err != nil {
		t.Fatalf("UpdateSessionBlock: %v", err)
	}

	_, after := queryLatestAuditSnapshot(t, db, "exam_session.update_block", "competency_exam_sessions")
	assertValidJSON(t, "UpdateSessionBlock after_snapshot", after)
}

// TestSnapshot_Exam_GenerateSession_HasAfterSnapshot verifies that generating
// an exam session writes a non-empty after_snapshot.
func TestSnapshot_Exam_GenerateSession_HasAfterSnapshot(t *testing.T) {
	cfg := testConfig(t)
	log := logger.New("text", "error")
	db, err := app.BootstrapFast(cfg, log)
	if err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	sched := newExamSched(t, db, log)

	var actorID string
	db.QueryRow(`SELECT id FROM users WHERE is_active=1 LIMIT 1`).Scan(&actorID) //nolint:errcheck

	tmpl, _ := sched.CreateTemplate(examdomain.CreateTemplate{
		Title: "Generate Snapshot", DurationMinutes: 60,
		PassingScore: 70, RoleScope: "nurse", WindowStart: "08:00", WindowEnd: "17:00",
	}, actorID, "", "")

	// 10:00 UTC five days out — inside the 08:00–17:00 window.
	_, _, err = sched.GenerateSession(examdomain.GenerateSessionCmd{
		TemplateID:   tmpl.ID,
		PlannedStart: time.Now().UTC().Truncate(24 * time.Hour).Add(120*time.Hour + 10*time.Hour),
		Room:         "gen-snap-room",
		ProctorID:    "",
	}, actorID, "", "")
	if err != nil {
		t.Fatalf("GenerateSession: %v", err)
	}

	_, after := queryLatestAuditSnapshot(t, db, "exam_session.generate", "competency_exam_sessions")
	assertValidJSON(t, "GenerateSession after_snapshot", after)
}

// ── Service delivery — alert creation ───────────────────────────────────────

// TestSnapshot_ServiceDelivery_CreateAlert_HasAfterSnapshot verifies that
// creating a service-delivery alert writes a non-empty after_snapshot.
func TestSnapshot_ServiceDelivery_CreateAlert_HasAfterSnapshot(t *testing.T) {
	cfg := testConfig(t)
	log := logger.New("text", "error")
	db, err := app.BootstrapFast(cfg, log)
	if err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	a := app.New(cfg, db, log)

	cookie := loginAs(t, a, "admin@careops.local", "Admin!234567")

	idemKey := "snap-alert-" + idgen.New()
	form := url.Values{}
	form.Set("resident_id", "res-001")
	form.Set("alert_type", "fall_risk")
	form.Set("severity", "medium")
	form.Set("message", "Snapshot test alert for before/after check")
	form.Set("_idempotency_key", idemKey)

	req := httptest.NewRequest(http.MethodPost, "/service-delivery/alerts",
		strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(&http.Cookie{Name: "careops_session", Value: cookie})
	resp, err := a.Test(req, -1)
	if err != nil {
		t.Fatalf("POST /service-delivery/alerts: %v", err)
	}
	if resp.StatusCode != http.StatusFound && resp.StatusCode != http.StatusSeeOther {
		body := readBody(t, resp)
		t.Fatalf("create alert: want redirect, got %d — %s", resp.StatusCode, body[:min(200, len(body))])
	}

	_, after := queryLatestAuditSnapshot(t, db, "create", "alert_event")
	assertValidJSON(t, "CreateAlert after_snapshot", after)

	var m map[string]any
	json.Unmarshal([]byte(after), &m) //nolint:errcheck
	if firstOf(m, "alert_type") == nil {
		t.Error("CreateAlert after_snapshot: expected alert_type key")
	}
	if firstOf(m, "resident_id") == nil {
		t.Error("CreateAlert after_snapshot: expected resident_id key")
	}
}

// ── Service delivery — checkpoint creation ───────────────────────────────────

// TestSnapshot_ServiceDelivery_CreateCheckpoint_HasAfterSnapshot verifies that
// creating a care quality checkpoint writes a non-empty after_snapshot.
func TestSnapshot_ServiceDelivery_CreateCheckpoint_HasAfterSnapshot(t *testing.T) {
	cfg := testConfig(t)
	log := logger.New("text", "error")
	db, err := app.BootstrapFast(cfg, log)
	if err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	a := app.New(cfg, db, log)

	cookie := loginAs(t, a, "admin@careops.local", "Admin!234567")

	idemKey := "snap-checkpoint-" + idgen.New()
	form := url.Values{}
	form.Set("resident_id", "res-001")
	form.Set("checkpoint_type", "pain_assessment")
	form.Set("score", "4")
	form.Set("notes", "Snapshot test checkpoint")
	form.Set("_idempotency_key", idemKey)

	req := httptest.NewRequest(http.MethodPost, "/service-delivery/checkpoints",
		strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(&http.Cookie{Name: "careops_session", Value: cookie})
	resp, err := a.Test(req, -1)
	if err != nil {
		t.Fatalf("POST /service-delivery/checkpoints: %v", err)
	}
	if resp.StatusCode != http.StatusFound && resp.StatusCode != http.StatusSeeOther {
		body := readBody(t, resp)
		t.Fatalf("create checkpoint: want redirect, got %d — %s", resp.StatusCode, body[:min(200, len(body))])
	}

	_, after := queryLatestAuditSnapshot(t, db, "create", "care_quality_checkpoint")
	assertValidJSON(t, "CreateCheckpoint after_snapshot", after)

	var m map[string]any
	json.Unmarshal([]byte(after), &m) //nolint:errcheck
	if firstOf(m, "checkpoint_type") == nil {
		t.Error("CreateCheckpoint after_snapshot: expected checkpoint_type key")
	}
	if firstOf(m, "resident_id") == nil {
		t.Error("CreateCheckpoint after_snapshot: expected resident_id key")
	}
	if firstOf(m, "score") == nil {
		t.Error("CreateCheckpoint after_snapshot: expected score key")
	}
}

// ── Reports — toggle active ──────────────────────────────────────────────────

// TestSnapshot_Reports_ToggleActive_HasBeforeAndAfter verifies that toggling a
// report definition writes both before_snapshot and after_snapshot capturing
// the is_active state change.
func TestSnapshot_Reports_ToggleActive_HasBeforeAndAfter(t *testing.T) {
	cfg := testConfig(t)
	log := logger.New("text", "error")
	db, err := app.BootstrapFast(cfg, log)
	if err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	svc := newReportSvc(t, db, log)
	var actorID string
	db.QueryRow(`SELECT id FROM users WHERE is_active=1 LIMIT 1`).Scan(&actorID) //nolint:errcheck

	def, err := svc.CreateDefinition(reportsdomain.CreateReportCmd{
		Name: "Toggle Snapshot Test", ReportType: "occupancy", OutputFormat: "csv",
	}, actorID, "", "")
	if err != nil {
		t.Fatalf("create definition: %v", err)
	}

	if err := svc.ToggleActive(def.ID, actorID, "", ""); err != nil {
		t.Fatalf("toggle active: %v", err)
	}

	before, after := queryLatestAuditSnapshot(t, db, "update", "scheduled_reports")
	assertValidJSON(t, "ToggleActive before_snapshot", before)
	assertValidJSON(t, "ToggleActive after_snapshot", after)

	var bm, am map[string]any
	json.Unmarshal([]byte(before), &bm) //nolint:errcheck
	json.Unmarshal([]byte(after), &am)  //nolint:errcheck

	beforeActive := firstOf(bm, "is_active")
	afterActive := firstOf(am, "is_active")
	if beforeActive == afterActive {
		t.Errorf("ToggleActive snapshots: before.is_active=%v should differ from after.is_active=%v", beforeActive, afterActive)
	}
	if firstOf(bm, "id") == nil {
		t.Error("ToggleActive before_snapshot: expected id key")
	}
	if firstOf(am, "name") == nil {
		t.Error("ToggleActive after_snapshot: expected name key")
	}
}

// ── Reports — run report ─────────────────────────────────────────────────────

// TestSnapshot_Reports_RunReport_HasAfterSnapshot verifies that running a
// report writes a non-empty after_snapshot with run metadata.
func TestSnapshot_Reports_RunReport_HasAfterSnapshot(t *testing.T) {
	cfg := testConfig(t)
	log := logger.New("text", "error")
	db, err := app.BootstrapFast(cfg, log)
	if err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	svc := newReportSvc(t, db, log)
	var actorID string
	db.QueryRow(`SELECT id FROM users WHERE is_active=1 LIMIT 1`).Scan(&actorID) //nolint:errcheck

	def, err := svc.CreateDefinition(reportsdomain.CreateReportCmd{
		Name: "RunReport Snapshot Test", ReportType: "occupancy", OutputFormat: "csv",
	}, actorID, "", "")
	if err != nil {
		t.Fatalf("create definition: %v", err)
	}

	run, err := svc.RunReport(def.ID, actorID, "", "")
	if err != nil {
		t.Fatalf("run report: %v", err)
	}
	_ = run

	_, after := queryLatestAuditSnapshot(t, db, "create", "report_runs")
	assertValidJSON(t, "RunReport after_snapshot", after)

	var m map[string]any
	json.Unmarshal([]byte(after), &m) //nolint:errcheck
	if firstOf(m, "report_type") == nil {
		t.Error("RunReport after_snapshot: expected report_type key")
	}
	if firstOf(m, "row_count") == nil {
		t.Error("RunReport after_snapshot: expected row_count key")
	}
	if firstOf(m, "file_name") == nil {
		t.Error("RunReport after_snapshot: expected file_name key")
	}
}

// ── Diagnostics — export package ─────────────────────────────────────────────

// TestSnapshot_Diagnostics_ExportPackage_HasAfterSnapshot verifies that
// generating a diagnostic ZIP writes a non-empty after_snapshot.
func TestSnapshot_Diagnostics_ExportPackage_HasAfterSnapshot(t *testing.T) {
	cfg := testConfig(t)
	log := logger.New("text", "error")
	db, err := app.BootstrapFast(cfg, log)
	if err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	svc := newDiagSvc(t, cfg, db, log)
	var actorID string
	db.QueryRow(`SELECT id FROM users WHERE is_active=1 LIMIT 1`).Scan(&actorID) //nolint:errcheck

	_, err = svc.ExportPackage(actorID, "", "test-corr-id")
	if err != nil {
		t.Fatalf("export package: %v", err)
	}

	_, after := queryLatestAuditSnapshot(t, db, "create", "diagnostic_exports")
	assertValidJSON(t, "ExportPackage after_snapshot", after)

	var m map[string]any
	json.Unmarshal([]byte(after), &m) //nolint:errcheck
	if firstOf(m, "id") == nil {
		t.Error("ExportPackage after_snapshot: expected id key")
	}
	if firstOf(m, "file_name") == nil {
		t.Error("ExportPackage after_snapshot: expected file_name key")
	}
}

// ── Config versions — take snapshot ──────────────────────────────────────────

// TestSnapshot_ConfigVersions_TakeSnapshot_HasAfterSnapshot verifies that
// taking a config snapshot writes a non-empty after_snapshot.
func TestSnapshot_ConfigVersions_TakeSnapshot_HasAfterSnapshot(t *testing.T) {
	cfg := testConfig(t)
	log := logger.New("text", "error")
	db, err := app.BootstrapFast(cfg, log)
	if err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	svc := newCfgSvc(t, cfg, db, log)
	var actorID string
	db.QueryRow(`SELECT id FROM users WHERE is_active=1 LIMIT 1`).Scan(&actorID) //nolint:errcheck

	_, err = svc.TakeSnapshot(actorID, "", "")
	if err != nil {
		t.Fatalf("take snapshot: %v", err)
	}

	_, after := queryLatestAuditSnapshot(t, db, "create", "config_snapshot")
	assertValidJSON(t, "TakeSnapshot after_snapshot", after)

	var m map[string]any
	json.Unmarshal([]byte(after), &m) //nolint:errcheck
	if firstOf(m, "file_name") == nil {
		t.Error("TakeSnapshot after_snapshot: expected file_name key")
	}
	if firstOf(m, "entry_count") == nil {
		t.Error("TakeSnapshot after_snapshot: expected entry_count key")
	}
}

// ── Config versions — rollback ────────────────────────────────────────────────

// TestSnapshot_ConfigVersions_Rollback_HasBeforeAndAfter verifies that rolling
// back to a snapshot writes both before_snapshot and after_snapshot.
func TestSnapshot_ConfigVersions_Rollback_HasBeforeAndAfter(t *testing.T) {
	cfg := testConfig(t)
	log := logger.New("text", "error")
	db, err := app.BootstrapFast(cfg, log)
	if err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	svc := newCfgSvc(t, cfg, db, log)
	var actorID string
	db.QueryRow(`SELECT id FROM users WHERE is_active=1 LIMIT 1`).Scan(&actorID) //nolint:errcheck

	// Take a snapshot first so we have something to roll back to.
	snapPath, err := svc.TakeSnapshot(actorID, "", "")
	if err != nil {
		t.Fatalf("take snapshot: %v", err)
	}
	snapFile := filepath.Base(snapPath)

	if err := svc.Rollback(snapFile, actorID, "", ""); err != nil {
		t.Fatalf("rollback: %v", err)
	}

	before, after := queryLatestAuditSnapshot(t, db, "update", "config_snapshot")
	assertValidJSON(t, "Rollback before_snapshot", before)
	assertValidJSON(t, "Rollback after_snapshot", after)

	var bm, am map[string]any
	json.Unmarshal([]byte(before), &bm) //nolint:errcheck
	json.Unmarshal([]byte(after), &am)  //nolint:errcheck

	if firstOf(bm, "live_entry_count") == nil {
		t.Error("Rollback before_snapshot: expected live_entry_count key")
	}
	if firstOf(am, "snapshot_file") == nil {
		t.Error("Rollback after_snapshot: expected snapshot_file key")
	}
	if firstOf(am, "restored_entry_count") == nil {
		t.Error("Rollback after_snapshot: expected restored_entry_count key")
	}
}

// ── Finance — export report ───────────────────────────────────────────────────

// TestSnapshot_Finance_ExportReport_HasAfterSnapshot verifies that generating
// a finance export writes a non-empty after_snapshot.
func TestSnapshot_Finance_ExportReport_HasAfterSnapshot(t *testing.T) {
	cfg := testConfig(t)
	log := logger.New("text", "error")
	db, err := app.BootstrapFast(cfg, log)
	if err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	svc := newFinanceSvc(t, db, log)
	var actorID string
	db.QueryRow(`SELECT id FROM users WHERE is_active=1 LIMIT 1`).Scan(&actorID) //nolint:errcheck

	params := financedomain.ExportParams{
		ReportType: "occupancy",
		Format:     "csv",
	}
	_, err = svc.ExportReport(params, actorID, "", "")
	if err != nil {
		t.Fatalf("export report: %v", err)
	}

	_, after := queryLatestAuditSnapshot(t, db, "create", "report_runs")
	assertValidJSON(t, "ExportReport after_snapshot", after)

	var m map[string]any
	json.Unmarshal([]byte(after), &m) //nolint:errcheck
	if firstOf(m, "report_type") == nil {
		t.Error("ExportReport after_snapshot: expected report_type key")
	}
	if firstOf(m, "format") == nil {
		t.Error("ExportReport after_snapshot: expected format key")
	}
	if firstOf(m, "row_count") == nil {
		t.Error("ExportReport after_snapshot: expected row_count key")
	}
	if firstOf(m, "file_name") == nil {
		t.Error("ExportReport after_snapshot: expected file_name key")
	}
}

// ── helpers ──────────────────────────────────────────────────────────────────

// firstOf returns the value for the first key found in m (case-sensitive),
// used to handle both camelCase and snake_case JSON keys from ToJSON.
func firstOf(m map[string]any, keys ...string) any {
	for _, k := range keys {
		if v, ok := m[k]; ok {
			return v
		}
	}
	return nil
}
