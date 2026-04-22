package api_tests

// Issue 2 — Resident-oriented audit search
//
// These tests verify the new resident_id filter on the audit log endpoint.
// They cover positive matches, no-match cases, and combinations with other
// filter parameters.  All tests operate via the HTTP layer so the full
// query-parameter → repository → SQL path is exercised.

import (
	"database/sql"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"careops/clinic/internal/app"
	"careops/clinic/internal/platform/logger"
	"careops/clinic/internal/shared/idgen"
)

// seedResidentAuditEntries inserts two audit entries linked to different
// residents via payments and returns (paymentIDRes1, paymentIDRes2).
func seedResidentAuditEntries(t *testing.T, db *sql.DB) (string, string) {
	t.Helper()

	// Ensure a shift exists.
	shiftID := "rat-shift-001"
	db.Exec(`INSERT OR IGNORE INTO settlement_shifts
		(id,shift_date,shift_name,opened_by,opened_at,status,created_at,updated_at)
		VALUES (?,?,?,?,datetime('now'),?,datetime('now'),datetime('now'))`,
		shiftID, "2025-03-01", "morning", "usr-finance", "open")

	p1 := idgen.New()
	p2 := idgen.New()
	db.Exec(`INSERT INTO payments
		(id,resident_id,shift_id,payment_method,amount_cents,currency,
		 description,recorded_by,status,row_version,recorded_at,created_at,updated_at)
		VALUES (?,'res-001',?,'cash',1100,'USD','rat-p1','usr-finance','completed',1,
			datetime('now'),datetime('now'),datetime('now'))`, p1, shiftID)
	db.Exec(`INSERT INTO payments
		(id,resident_id,shift_id,payment_method,amount_cents,currency,
		 description,recorded_by,status,row_version,recorded_at,created_at,updated_at)
		VALUES (?,'res-002',?,'cash',2200,'USD','rat-p2','usr-finance','completed',1,
			datetime('now'),datetime('now'),datetime('now'))`, p2, shiftID)

	// Seed audit log entries pointing to those payment IDs.
	db.Exec(`INSERT INTO audit_logs
		(id,user_id,action,entity_type,entity_id,details,outcome,ip_address,request_id,
		 occurred_at,created_at)
		VALUES (?,?,?,?,?,?,?,?,?,datetime('now'),datetime('now'))`,
		idgen.New(), "usr-finance", "create", "payments", p1, "{}", "success", "", "")
	db.Exec(`INSERT INTO audit_logs
		(id,user_id,action,entity_type,entity_id,details,outcome,ip_address,request_id,
		 occurred_at,created_at)
		VALUES (?,?,?,?,?,?,?,?,?,datetime('now'),datetime('now'))`,
		idgen.New(), "usr-finance", "create", "payments", p2, "{}", "success", "", "")

	return p1, p2
}

// TestAudit_ResidentFilter_PositiveMatch verifies that resident_id=res-001
// returns audit entries whose entity_id matches a payment linked to res-001.
func TestAudit_ResidentFilter_PositiveMatch(t *testing.T) {
	cfg := testConfig(t)
	log := logger.New("text", "error")
	db, err := app.BootstrapFast(cfg, log)
	if err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	a := app.New(cfg, db, log)

	p1, _ := seedResidentAuditEntries(t, db)
	adminCookie := loginAs(t, a, "admin@careops.local", "Admin!234567")

	resp := getWithSession(t, a, "/audit?resident_id=res-001", adminCookie)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /audit?resident_id=res-001: want 200, got %d", resp.StatusCode)
	}
	body := readBody(t, resp)

	// The page must contain the payment ID for res-001.
	if !strings.Contains(body, p1) {
		t.Errorf("audit resident filter: expected entity_id %q (res-001 payment) in response", p1)
	}
}

// TestAudit_ResidentFilter_ExcludesOtherResident verifies that resident_id=res-001
// does NOT include audit entries for res-002's payments.
func TestAudit_ResidentFilter_ExcludesOtherResident(t *testing.T) {
	cfg := testConfig(t)
	log := logger.New("text", "error")
	db, err := app.BootstrapFast(cfg, log)
	if err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	a := app.New(cfg, db, log)

	_, p2 := seedResidentAuditEntries(t, db)
	adminCookie := loginAs(t, a, "admin@careops.local", "Admin!234567")

	resp := getWithSession(t, a, "/audit?resident_id=res-001", adminCookie)
	body := readBody(t, resp)

	if strings.Contains(body, p2) {
		t.Errorf("audit resident filter: entity_id %q (res-002 payment) must not appear when filtering for res-001", p2)
	}
}

// TestAudit_ResidentFilter_NoMatch verifies that a non-existent resident ID
// returns an empty result set without error.
func TestAudit_ResidentFilter_NoMatch(t *testing.T) {
	a := buildTestApp(t)
	cookie := loginAs(t, a, "admin@careops.local", "Admin!234567")

	resp := getWithSession(t, a, "/audit?resident_id=does-not-exist-resident", cookie)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /audit?resident_id=no-such-resident: want 200, got %d", resp.StatusCode)
	}
	body := readBody(t, resp)
	// Page should render normally with the "no entries" message.
	if !strings.Contains(body, "No audit entries") && !strings.Contains(body, "Audit Log") {
		t.Error("audit no-match resident filter: page should still render the audit table")
	}
}

// TestAudit_ResidentFilter_CombinedWithDateRange verifies that combining
// resident_id with date_from/date_to applies both constraints.
func TestAudit_ResidentFilter_CombinedWithDateRange(t *testing.T) {
	a := buildTestApp(t)
	cookie := loginAs(t, a, "admin@careops.local", "Admin!234567")

	// A very narrow future date range — effectively returns zero rows.
	params := url.Values{}
	params.Set("resident_id", "res-001")
	params.Set("date_from", "2099-01-01")
	params.Set("date_to", "2099-01-02")
	resp := getWithSession(t, a, "/audit?"+params.Encode(), cookie)

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("combined resident+date filter: want 200, got %d", resp.StatusCode)
	}
	body := readBody(t, resp)
	if !strings.Contains(body, "Audit Log") {
		t.Error("combined filter: page should still render the audit table heading")
	}
	if !strings.Contains(body, "No audit entries") {
		t.Error("combined filter with future date range: expected empty result set")
	}
}

// TestAudit_ResidentFilter_CombinedWithEntityType verifies that combining
// resident_id with entity_type limits results to that type for that resident.
func TestAudit_ResidentFilter_CombinedWithEntityType(t *testing.T) {
	cfg := testConfig(t)
	log := logger.New("text", "error")
	db, err := app.BootstrapFast(cfg, log)
	if err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	a := app.New(cfg, db, log)

	p1, _ := seedResidentAuditEntries(t, db)
	adminCookie := loginAs(t, a, "admin@careops.local", "Admin!234567")

	// Filter by resident res-001 AND entity_type=payments.
	params := url.Values{}
	params.Set("resident_id", "res-001")
	params.Set("entity_type", "payments")
	resp := getWithSession(t, a, "/audit?"+params.Encode(), adminCookie)

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("combined entity+resident filter: want 200, got %d", resp.StatusCode)
	}
	body := readBody(t, resp)
	if !strings.Contains(body, p1) {
		t.Errorf("combined filter: expected entity_id %q (res-001 payment) in response", p1)
	}
}

// TestAudit_ResidentFilter_PageIncludesResidentIDInput verifies that the audit
// page renders a resident_id filter input field, showing the value was persisted.
func TestAudit_ResidentFilter_PageIncludesResidentIDInput(t *testing.T) {
	a := buildTestApp(t)
	cookie := loginAs(t, a, "admin@careops.local", "Admin!234567")

	resp := getWithSession(t, a, "/audit?resident_id=res-001", cookie)
	body := readBody(t, resp)

	// The form should include a resident_id input with the submitted value.
	if !strings.Contains(body, `name="resident_id"`) {
		t.Error("audit page: should contain resident_id input field")
	}
	if !strings.Contains(body, `value="res-001"`) {
		t.Error("audit page: resident_id input should preserve submitted value")
	}
	// Active filters banner should show the resident badge.
	if !strings.Contains(body, "resident: res-001") {
		t.Error("audit page: active filters banner should show resident filter badge")
	}
}

// TestAudit_ResidentFilter_ServiceOrderMatch verifies that service_orders
// entries for a resident also appear when filtering by that resident_id.
func TestAudit_ResidentFilter_ServiceOrderMatch(t *testing.T) {
	cfg := testConfig(t)
	log := logger.New("text", "error")
	db, err := app.BootstrapFast(cfg, log)
	if err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	a := app.New(cfg, db, log)

	// so-106 is the seeded service order for res-001.
	// Seed an audit entry pointing to so-106.
	db.Exec(`INSERT INTO audit_logs
		(id,user_id,action,entity_type,entity_id,details,outcome,ip_address,request_id,
		 occurred_at,created_at)
		VALUES (?,?,?,?,?,?,?,?,?,datetime('now'),datetime('now'))`,
		idgen.New(), "usr-nurse", "create", "service_orders", "so-106", "{}", "success", "", "")

	adminCookie := loginAs(t, a, "admin@careops.local", "Admin!234567")
	resp := getWithSession(t, a, "/audit?resident_id=res-001&entity_type=service_orders", adminCookie)

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /audit with resident+service_orders filter: want 200, got %d", resp.StatusCode)
	}
	body := readBody(t, resp)
	if !strings.Contains(body, "so-106") {
		t.Error("audit resident filter: expected so-106 (res-001 service order) in response")
	}
}
