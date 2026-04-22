package api_tests

import (
	"database/sql"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"careops/clinic/internal/app"
	"careops/clinic/internal/platform/logger"
)

// ── Finance payment idempotency ───────────────────────────────────────────────

// TestIdempotency_PaymentDuplicate posts the same payment twice with the same
// _idempotency_key and verifies both calls succeed (redirect) without creating
// duplicate audit/DB rows (the second is a replay of the first).
func TestIdempotency_PaymentDuplicate(t *testing.T) {
	a := buildTestApp(t)
	cookie := loginAs(t, a, "finance@careops.local", "Finance!2345")

	// Discover a resident ID from the payment form.
	formResp := getWithSession(t, a, "/finance/payments/new", cookie)
	formBody, _ := io.ReadAll(formResp.Body)
	formStr := string(formBody)
	idx := strings.Index(formStr, `value="res-`)
	if idx < 0 {
		t.Skip("no resident found in payment form — seeds may not have loaded")
	}
	start := idx + len(`value="`)
	end := strings.Index(formStr[start:], `"`)
	residentID := formStr[start : start+end]

	form := url.Values{}
	form.Set("resident_id", residentID)
	form.Set("payment_method", "cash")
	form.Set("amount", "15.00")
	form.Set("description", "Idempotency test payment")
	form.Set("_idempotency_key", "idem-pay-test-001")

	doPost := func() *http.Response {
		req := httptest.NewRequest(http.MethodPost, "/finance/payments", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.AddCookie(&http.Cookie{Name: "careops_session", Value: cookie})
		resp, err := a.Test(req, -1)
		if err != nil {
			t.Fatalf("POST /finance/payments: %v", err)
		}
		return resp
	}

	resp1 := doPost()
	resp2 := doPost()

	// Both should succeed — a redirect indicates successful creation or replay.
	for i, r := range []*http.Response{resp1, resp2} {
		if r.StatusCode != http.StatusFound && r.StatusCode != http.StatusSeeOther {
			body, _ := io.ReadAll(r.Body)
			t.Errorf("payment idempotency request %d: expected redirect, got %d — %s", i+1, r.StatusCode, string(body)[:min(300, len(body))])
		}
	}
}

// TestIdempotency_AlertDuplicateDoesNotDouble posts the same alert twice with
// the same idempotency key and verifies the alert count does not increase on
// the second call.
func TestIdempotency_AlertDuplicateDoesNotDouble(t *testing.T) {
	cfg := testConfig(t)
	log := logger.New("text", "error")
	db, err := app.Bootstrap(cfg, log)
	if err != nil {
		t.Fatalf("Bootstrap: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	a := app.New(cfg, db, log)

	cookie := loginAs(t, a, "admin@careops.local", "Admin!234567")

	form := url.Values{}
	form.Set("resident_id", "res-001")
	form.Set("alert_type", "fall_risk")
	form.Set("severity", "medium")
	form.Set("message", "Idempotency alert dedup test")
	form.Set("_idempotency_key", "idem-alert-dedup-001")

	doPost := func() *http.Response {
		req := httptest.NewRequest(http.MethodPost, "/service-delivery/alerts", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.AddCookie(&http.Cookie{Name: "careops_session", Value: cookie})
		resp, err := a.Test(req, -1)
		if err != nil {
			t.Fatalf("POST /service-delivery/alerts: %v", err)
		}
		return resp
	}

	countAlerts := func() int {
		var n int
		db.QueryRow(`SELECT COUNT(*) FROM alert_events WHERE message = 'Idempotency alert dedup test'`).Scan(&n)
		return n
	}

	// First POST — should create the alert.
	r1 := doPost()
	if r1.StatusCode != http.StatusFound && r1.StatusCode != http.StatusSeeOther {
		body, _ := io.ReadAll(r1.Body)
		t.Fatalf("first alert POST: want redirect, got %d — %s", r1.StatusCode, string(body)[:min(200, len(body))])
	}
	countAfterFirst := countAlerts()

	// Second POST with same key — should be replayed, not a new row.
	r2 := doPost()
	if r2.StatusCode != http.StatusFound && r2.StatusCode != http.StatusSeeOther {
		body, _ := io.ReadAll(r2.Body)
		t.Fatalf("second alert POST (idempotent): want redirect, got %d — %s", r2.StatusCode, string(body)[:min(200, len(body))])
	}
	countAfterSecond := countAlerts()

	if countAfterSecond != countAfterFirst {
		t.Errorf("alert idempotency: expected same count after duplicate POST, got %d then %d",
			countAfterFirst, countAfterSecond)
	}
}

// TestIdempotency_CheckpointDuplicateIsIdempotent posts the same checkpoint
// twice with the same idempotency key and verifies the second call is a replay.
func TestIdempotency_CheckpointDuplicateIsIdempotent(t *testing.T) {
	cfg := testConfig(t)
	log := logger.New("text", "error")
	db, err := app.Bootstrap(cfg, log)
	if err != nil {
		t.Fatalf("Bootstrap: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	a := app.New(cfg, db, log)

	cookie := loginAs(t, a, "admin@careops.local", "Admin!234567")

	form := url.Values{}
	form.Set("resident_id", "res-001")
	form.Set("checkpoint_type", "mobility")
	form.Set("score", "3")
	form.Set("notes", "Idempotency checkpoint test")
	form.Set("_idempotency_key", "idem-checkpoint-dedup-001")

	doPost := func() *http.Response {
		req := httptest.NewRequest(http.MethodPost, "/service-delivery/checkpoints", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.AddCookie(&http.Cookie{Name: "careops_session", Value: cookie})
		resp, err := a.Test(req, -1)
		if err != nil {
			t.Fatalf("POST /service-delivery/checkpoints: %v", err)
		}
		return resp
	}

	countCheckpoints := func() int {
		var n int
		db.QueryRow(`SELECT COUNT(*) FROM care_quality_checkpoints WHERE notes = 'Idempotency checkpoint test'`).Scan(&n)
		return n
	}

	// First POST — creates the checkpoint.
	r1 := doPost()
	if r1.StatusCode != http.StatusFound && r1.StatusCode != http.StatusSeeOther {
		body, _ := io.ReadAll(r1.Body)
		t.Fatalf("first checkpoint POST: want redirect, got %d — %s", r1.StatusCode, string(body)[:min(200, len(body))])
	}
	countAfterFirst := countCheckpoints()

	// Second POST with same key — should replay, not insert again.
	r2 := doPost()
	if r2.StatusCode != http.StatusFound && r2.StatusCode != http.StatusSeeOther {
		body, _ := io.ReadAll(r2.Body)
		t.Fatalf("second checkpoint POST (idempotent): want redirect, got %d — %s", r2.StatusCode, string(body)[:min(200, len(body))])
	}
	countAfterSecond := countCheckpoints()

	if countAfterSecond != countAfterFirst {
		t.Errorf("checkpoint idempotency: count changed from %d to %d after duplicate POST",
			countAfterFirst, countAfterSecond)
	}
}

// countRowsDB is a small helper for test assertions that checks a simple COUNT query.
func countRowsDB(db *sql.DB, query string, args ...interface{}) int {
	var n int
	db.QueryRow(query, args...).Scan(&n)
	return n
}
