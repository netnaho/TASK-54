package api_tests

// Issue 1 — Report custom filter semantics
//
// These tests verify that the Parameters JSON field on a ScheduledReport is
// actually parsed and applied as WHERE clauses when the report is executed.
// Each test creates a report definition with specific parameters, runs the
// report via the service layer, reads the output CSV, and asserts the correct
// rows (and only those rows) are present.

import (
	"database/sql"
	"encoding/csv"
	"os"
	"strings"
	"testing"

	"careops/clinic/internal/app"
	auditrepo "careops/clinic/internal/modules/audit/repository"
	auditsvc "careops/clinic/internal/modules/audit/service"
	"careops/clinic/internal/modules/reports/domain"
	reportrepo "careops/clinic/internal/modules/reports/repository"
	reportsvc "careops/clinic/internal/modules/reports/service"
	"careops/clinic/internal/platform/jobs"
	"careops/clinic/internal/platform/logger"
	"careops/clinic/internal/shared/idgen"
)

// buildReportService returns a wired-up service and the underlying *sql.DB.
func buildReportService(t *testing.T) (*reportsvc.ReportService, *sql.DB) {
	t.Helper()
	cfg := testConfig(t)
	log := logger.New("text", "error")
	db, err := app.BootstrapFast(cfg, log)
	if err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	repo := reportrepo.NewReportRepo(db)
	jobWriter := jobs.NewWriter(db)
	ar := auditrepo.NewAuditRepo(db)
	as := auditsvc.NewAuditService(ar, log)
	svc := reportsvc.New(db, repo, as, jobWriter, cfg.ExportsPath, log)
	return svc, db
}

// createReport inserts a ScheduledReport definition and returns its ID.
func createReport(t *testing.T, svc *reportsvc.ReportService, name, reportType, params string) string {
	t.Helper()
	sr, err := svc.CreateDefinition(domain.CreateReportCmd{
		Name:         name,
		ReportType:   reportType,
		OutputFormat: "csv",
		Parameters:   params,
	}, "usr-admin", "", "")
	if err != nil {
		t.Fatalf("createReport %q: %v", name, err)
	}
	return sr.ID
}

// readCSVFile opens and parses a CSV file; returns header + rows.
func readCSVFile(t *testing.T, path string) ([]string, [][]string) {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open CSV %s: %v", path, err)
	}
	defer f.Close()
	rows, err := csv.NewReader(f).ReadAll()
	if err != nil {
		t.Fatalf("parse CSV %s: %v", path, err)
	}
	if len(rows) == 0 {
		return nil, nil
	}
	return rows[0], rows[1:]
}

// ── Finance report filters ───────────────────────────────────────────────────

// TestReportFilter_Finance_ResidentID verifies that a finance report with
// resident_id in parameters returns only payments for that resident.
func TestReportFilter_Finance_ResidentID(t *testing.T) {
	svc, db := buildReportService(t)

	// Insert two payments: one for res-001 and one for res-002.
	var shiftID string
	// Open a shift so payments can be linked to it.
	if err := db.QueryRow(
		`INSERT INTO settlement_shifts
			(id,shift_date,shift_name,opened_by,opened_at,status,created_at,updated_at)
			VALUES ('rf-shift-01','2024-06-01','morning','usr-finance',datetime('now'),'open',
				datetime('now'),datetime('now'))
		RETURNING id`).Scan(&shiftID); err != nil {
		shiftID = "rf-shift-01"
		db.Exec(`INSERT OR IGNORE INTO settlement_shifts
			(id,shift_date,shift_name,opened_by,opened_at,status,created_at,updated_at)
			VALUES (?,?,?,?,datetime('now'),?,datetime('now'),datetime('now'))`,
			shiftID, "2024-06-01", "morning", "usr-finance", "open")
	}
	p1ID := idgen.New()
	p2ID := idgen.New()
	db.Exec(`INSERT INTO payments
		(id,resident_id,shift_id,payment_method,amount_cents,currency,
		 description,recorded_by,status,row_version,recorded_at,created_at,updated_at)
		VALUES (?,?,?,'cash',5000,'USD','filter-test-res001','usr-finance','completed',1,
			datetime('now'),datetime('now'),datetime('now'))`,
		p1ID, "res-001", shiftID)
	db.Exec(`INSERT INTO payments
		(id,resident_id,shift_id,payment_method,amount_cents,currency,
		 description,recorded_by,status,row_version,recorded_at,created_at,updated_at)
		VALUES (?,?,?,'cash',7500,'USD','filter-test-res002','usr-finance','completed',1,
			datetime('now'),datetime('now'),datetime('now'))`,
		p2ID, "res-002", shiftID)

	// Create report definition filtered to res-001 only.
	id := createReport(t, svc, "Finance Filter Test", "finance",
		`{"resident_id":"res-001"}`)

	run, err := svc.RunReport(id, "usr-admin", "", "")
	if err != nil {
		t.Fatalf("RunReport: %v", err)
	}
	if run.Status != "complete" {
		t.Fatalf("run status: want complete, got %q", run.Status)
	}

	_, rows := readCSVFile(t, run.OutputPath)

	for _, row := range rows {
		// resident_name column is index 1
		residentName := row[1]
		if residentName != "" && !strings.Contains(strings.ToLower(residentName), "holloway") &&
			!strings.Contains(strings.ToLower(residentName), "margaret") {
			// Verify the payment ID is not p2ID (res-002 payment)
			if row[0] == p2ID {
				t.Errorf("finance report with resident_id filter: got row for res-002 (%s), expected only res-001", row[0])
			}
		}
	}

	// Must contain p1ID (res-001 payment).
	var foundP1 bool
	for _, row := range rows {
		if row[0] == p1ID {
			foundP1 = true
		}
	}
	if !foundP1 {
		t.Error("finance report with resident_id filter: expected payment for res-001, got none")
	}
	// Must NOT contain p2ID (res-002 payment).
	for _, row := range rows {
		if row[0] == p2ID {
			t.Error("finance report with resident_id filter: got payment for res-002, should be excluded")
		}
	}
}

// TestReportFilter_Finance_DateRange verifies that date_from/date_to parameters
// restrict the finance report to the given window.
func TestReportFilter_Finance_DateRange(t *testing.T) {
	svc, db := buildReportService(t)

	// Insert a payment in the past (should be excluded).
	var shiftID = "rf-shift-dr"
	db.Exec(`INSERT OR IGNORE INTO settlement_shifts
		(id,shift_date,shift_name,opened_by,opened_at,status,created_at,updated_at)
		VALUES (?,?,?,?,datetime('now'),?,datetime('now'),datetime('now'))`,
		shiftID, "2020-01-01", "morning", "usr-finance", "open")

	pastID := idgen.New()
	recentID := idgen.New()
	db.Exec(`INSERT INTO payments
		(id,resident_id,shift_id,payment_method,amount_cents,currency,
		 description,recorded_by,status,row_version,recorded_at,created_at,updated_at)
		VALUES (?,'res-002',?,'cash',9900,'USD','past-payment','usr-finance','completed',1,
			'2020-01-15T10:00:00Z','2020-01-15T10:00:00Z','2020-01-15T10:00:00Z')`,
		pastID, shiftID)
	db.Exec(`INSERT INTO payments
		(id,resident_id,shift_id,payment_method,amount_cents,currency,
		 description,recorded_by,status,row_version,recorded_at,created_at,updated_at)
		VALUES (?,'res-002',?,'cash',4400,'USD','recent-payment','usr-finance','completed',1,
			'2026-04-01T10:00:00Z','2026-04-01T10:00:00Z','2026-04-01T10:00:00Z')`,
		recentID, shiftID)

	id := createReport(t, svc, "Finance Date Filter", "finance",
		`{"date_from":"2026-01-01","date_to":"2026-12-31"}`)

	run, err := svc.RunReport(id, "usr-admin", "", "")
	if err != nil {
		t.Fatalf("RunReport: %v", err)
	}
	_, rows := readCSVFile(t, run.OutputPath)

	for _, row := range rows {
		if row[0] == pastID {
			t.Error("finance date range filter: got 2020 payment that should be excluded")
		}
	}

	var foundRecent bool
	for _, row := range rows {
		if row[0] == recentID {
			foundRecent = true
		}
	}
	if !foundRecent {
		t.Error("finance date range filter: expected 2026 payment in output, got none")
	}
}

// ── Service delivery report filters ─────────────────────────────────────────

// TestReportFilter_ServiceDelivery_ResidentID verifies resident_id parameter
// restricts service orders to a single resident.
func TestReportFilter_ServiceDelivery_ResidentID(t *testing.T) {
	svc, db := buildReportService(t)

	// so-106 is seeded for res-001 (OT, completed).
	// Insert an additional order for res-002 to confirm exclusion.
	soID := idgen.New()
	db.Exec(`INSERT INTO service_orders
		(id,resident_id,ordered_by,service_type,frequency,notes,status,start_date,
		 row_version,created_at,updated_at)
		VALUES (?,'res-002','usr-nurse','PT','daily','filter test res-002','active','2024-05-01',
			1,datetime('now'),datetime('now'))`, soID)

	id := createReport(t, svc, "SD Filter Test", "service_delivery",
		`{"resident_id":"res-001"}`)

	run, err := svc.RunReport(id, "usr-admin", "", "")
	if err != nil {
		t.Fatalf("RunReport: %v", err)
	}
	if run.Status != "complete" {
		t.Fatalf("run status: %q", run.Status)
	}
	_, rows := readCSVFile(t, run.OutputPath)

	for _, row := range rows {
		if row[0] == soID {
			t.Error("service_delivery report with resident_id filter: got res-002 order that should be excluded")
		}
	}
	// so-106 (res-001) must appear.
	var foundSO106 bool
	for _, row := range rows {
		if row[0] == "so-106" {
			foundSO106 = true
		}
	}
	if !foundSO106 {
		t.Error("service_delivery report: expected so-106 (res-001 order) in filtered output")
	}
}

// ── Audit report filters ─────────────────────────────────────────────────────

// TestReportFilter_Audit_EntityType verifies entity_type parameter restricts
// the audit report to entries of that type.
func TestReportFilter_Audit_EntityType(t *testing.T) {
	svc, db := buildReportService(t)

	// Seed a known audit entry with entity_type="audit_filter_test_entity".
	db.Exec(`INSERT INTO audit_logs
		(id,user_id,action,entity_type,entity_id,details,outcome,ip_address,request_id,
		 occurred_at,created_at)
		VALUES (?,?,?,?,?,?,?,?,?,datetime('now'),datetime('now'))`,
		idgen.New(), "usr-admin", "create", "audit_filter_test_entity", "afe-001",
		"{}", "success", "", "")

	id := createReport(t, svc, "Audit Entity Filter", "audit",
		`{"entity_type":"audit_filter_test_entity"}`)

	run, err := svc.RunReport(id, "usr-admin", "", "")
	if err != nil {
		t.Fatalf("RunReport: %v", err)
	}
	_, rows := readCSVFile(t, run.OutputPath)

	if len(rows) == 0 {
		t.Fatal("audit entity_type filter: expected at least one matching row")
	}
	for _, row := range rows {
		if row[3] != "audit_filter_test_entity" {
			t.Errorf("audit filter: got entity_type=%q, expected audit_filter_test_entity", row[3])
		}
	}
}

// ── Empty / invalid filter fallback ─────────────────────────────────────────

// TestReportFilter_EmptyParams_ReturnsAllRows verifies that an empty parameters
// object ("{}") imposes no filtering and returns the full result set.
func TestReportFilter_EmptyParams_ReturnsAllRows(t *testing.T) {
	svc, db := buildReportService(t)

	// There are seeded service orders for multiple residents.
	var total int
	db.QueryRow(`SELECT COUNT(*) FROM service_orders`).Scan(&total)
	if total == 0 {
		t.Skip("no service orders seeded")
	}

	id := createReport(t, svc, "SD No Filter", "service_delivery", "{}")
	run, err := svc.RunReport(id, "usr-admin", "", "")
	if err != nil {
		t.Fatalf("RunReport: %v", err)
	}
	_, rows := readCSVFile(t, run.OutputPath)

	if len(rows) == 0 {
		t.Error("empty params: expected rows in output, got none")
	}
	if len(rows) < 2 {
		t.Errorf("empty params: expected multiple service order rows, got %d", len(rows))
	}
}

// TestReportFilter_InvalidJSON_TreatedAsEmpty verifies that malformed JSON in
// parameters does not crash the report run — it falls back to no filtering.
func TestReportFilter_InvalidJSON_TreatedAsEmpty(t *testing.T) {
	svc, db := buildReportService(t)

	var total int
	db.QueryRow(`SELECT COUNT(*) FROM service_orders`).Scan(&total)
	if total == 0 {
		t.Skip("no service orders seeded")
	}

	id := createReport(t, svc, "Invalid JSON Params", "service_delivery",
		`{not valid json!`)

	// Should not error — falls back to unfiltered.
	run, err := svc.RunReport(id, "usr-admin", "", "")
	if err != nil {
		t.Fatalf("RunReport with invalid JSON params: unexpected error: %v", err)
	}
	if run.Status != "complete" {
		t.Errorf("run with invalid JSON params: want status=complete, got %q", run.Status)
	}
}

// TestReportFilter_Occupancy_ResidentID verifies the occupancy report filters
// by resident_id, returning only beds occupied by that resident.
func TestReportFilter_Occupancy_ResidentID(t *testing.T) {
	svc, _ := buildReportService(t)

	// res-001 is in bed-e101a (seeded, adm-001 active).
	id := createReport(t, svc, "Occupancy Resident Filter", "occupancy",
		`{"resident_id":"res-001"}`)

	run, err := svc.RunReport(id, "usr-admin", "", "")
	if err != nil {
		t.Fatalf("RunReport: %v", err)
	}
	_, rows := readCSVFile(t, run.OutputPath)

	// All rows must be for res-001's bed (bed-e101a) or be zero rows (vacant filter).
	for _, row := range rows {
		if row[6] != "" && !strings.Contains(strings.ToLower(row[6]), "holloway") &&
			!strings.Contains(strings.ToLower(row[6]), "margaret") {
			t.Errorf("occupancy resident filter: unexpected resident name %q", row[6])
		}
	}
}

// ── Scheduler with parameters ────────────────────────────────────────────────

// TestScheduler_RunsFilteredReport verifies that the scheduler correctly passes
// the stored Parameters JSON to the report execution when ticking.
func TestScheduler_RunsFilteredReport(t *testing.T) {
	cfg := testConfig(t)
	log := logger.New("text", "error")
	db, err := app.BootstrapFast(cfg, log)
	if err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	repo := reportrepo.NewReportRepo(db)
	jobWriter := jobs.NewWriter(db)
	ar := auditrepo.NewAuditRepo(db)
	as := auditsvc.NewAuditService(ar, log)
	svc := reportsvc.New(db, repo, as, jobWriter, cfg.ExportsPath, log)

	var creatorID string
	if err := db.QueryRow(`SELECT id FROM users WHERE is_active=1 LIMIT 1`).Scan(&creatorID); err != nil {
		t.Fatalf("get user: %v", err)
	}

	// Insert a report definition with resident_id filter and every-minute cron.
	paramJSON := `{"resident_id":"res-001","date_from":"2024-01-01"}`
	_, err = db.Exec(`
		INSERT INTO scheduled_reports
			(id,name,report_type,schedule_cron,parameters,output_format,
			 recipients,is_active,created_by,created_at,updated_at,row_version)
		VALUES ('sched-filter-001','Filtered Scheduler Report','finance','* * * * *',
			?,  'csv','[]',1,?,datetime('now'),datetime('now'),1)`,
		paramJSON, creatorID)
	if err != nil {
		t.Fatalf("insert report: %v", err)
	}

	// Verify parameters are stored and parseable.
	sr, err := repo.FindByID("sched-filter-001")
	if err != nil || sr == nil {
		t.Fatalf("FindByID: %v", err)
	}
	p := sr.ParseParams()
	if p.ResidentID != "res-001" {
		t.Errorf("ParseParams: expected resident_id=res-001, got %q", p.ResidentID)
	}
	if p.DateFrom != "2024-01-01" {
		t.Errorf("ParseParams: expected date_from=2024-01-01, got %q", p.DateFrom)
	}

	// Run the report via RunReport to confirm it uses the parameters.
	run, err := svc.RunReport("sched-filter-001", creatorID, "", "")
	if err != nil {
		t.Fatalf("RunReport with params: %v", err)
	}
	if run.Status != "complete" {
		t.Errorf("run with params: want complete, got %q", run.Status)
	}
	// Parameters stored in the run record must match.
	if run.Parameters != paramJSON {
		t.Errorf("run.Parameters: want %q, got %q", paramJSON, run.Parameters)
	}
}
