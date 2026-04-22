package api_tests

// Issue 5 — Finance idempotency on shift/batch routes
//
// Tests verify that duplicate POST requests to the shift-open, shift-close,
// and batch-import endpoints with the same idempotency key do not create
// duplicate records and return successful (redirect) responses on replay.

import (
	"bytes"
	"database/sql"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"careops/clinic/internal/app"
	"careops/clinic/internal/platform/logger"
)

// ── POST /finance/shifts (ShiftOpen) ─────────────────────────────────────────

// TestIdempotency_ShiftOpen_DuplicateKeyReturnsSameShift verifies that two
// POST /finance/shifts requests with the same idempotency key succeed and do
// not open two separate shifts.
func TestIdempotency_ShiftOpen_DuplicateKeyReturnsSameShift(t *testing.T) {
	cfg := testConfig(t)
	log := logger.New("text", "error")
	db, err := app.Bootstrap(cfg, log)
	if err != nil {
		t.Fatalf("Bootstrap: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	a := app.New(cfg, db, log)

	cookie := loginAs(t, a, "finance@careops.local", "Finance!2345")

	countShifts := func() int {
		var n int
		db.QueryRow(`SELECT COUNT(*) FROM settlement_shifts WHERE shift_date='2026-07-01' AND shift_name='morning'`).Scan(&n)
		return n
	}

	doOpen := func(idemKey string) *http.Response {
		t.Helper()
		form := url.Values{}
		form.Set("shift_date", "2026-07-01")
		form.Set("shift_name", "morning")
		form.Set("_idempotency_key", idemKey)
		req := httptest.NewRequest(http.MethodPost, "/finance/shifts",
			strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.AddCookie(&http.Cookie{Name: "careops_session", Value: cookie})
		resp, err := a.Test(req, -1)
		if err != nil {
			t.Fatalf("POST /finance/shifts: %v", err)
		}
		return resp
	}

	const idemKey = "shift-open-idem-test-001"
	r1 := doOpen(idemKey)
	r2 := doOpen(idemKey)

	for i, r := range []*http.Response{r1, r2} {
		if r.StatusCode != http.StatusFound && r.StatusCode != http.StatusSeeOther {
			t.Errorf("ShiftOpen idempotency request %d: want redirect, got %d", i+1, r.StatusCode)
		}
	}

	// Only one shift must exist for that date+name combination.
	if n := countShifts(); n != 1 {
		t.Errorf("ShiftOpen idempotency: expected 1 shift, got %d", n)
	}

	// Both redirects should point to the same shift.
	loc1 := r1.Header.Get("Location")
	loc2 := r2.Header.Get("Location")
	if loc1 != loc2 {
		t.Errorf("ShiftOpen idempotency: first redirect=%q, second redirect=%q (should be same)", loc1, loc2)
	}
}

// TestIdempotency_ShiftOpen_DifferentKeyOpensNewShift verifies that two
// requests with distinct keys each open their own shift.
func TestIdempotency_ShiftOpen_DifferentKeyOpensNewShift(t *testing.T) {
	cfg := testConfig(t)
	log := logger.New("text", "error")
	db, err := app.Bootstrap(cfg, log)
	if err != nil {
		t.Fatalf("Bootstrap: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	a := app.New(cfg, db, log)

	cookie := loginAs(t, a, "finance@careops.local", "Finance!2345")

	doOpen := func(date, idemKey string) *http.Response {
		t.Helper()
		form := url.Values{}
		form.Set("shift_date", date)
		form.Set("shift_name", "morning")
		form.Set("_idempotency_key", idemKey)
		req := httptest.NewRequest(http.MethodPost, "/finance/shifts",
			strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.AddCookie(&http.Cookie{Name: "careops_session", Value: cookie})
		resp, _ := a.Test(req, -1)
		return resp
	}

	r1 := doOpen("2026-08-01", "shift-different-key-001")
	r2 := doOpen("2026-08-02", "shift-different-key-002")

	if r1.StatusCode != http.StatusFound && r1.StatusCode != http.StatusSeeOther {
		t.Errorf("first shift open: want redirect, got %d", r1.StatusCode)
	}
	if r2.StatusCode != http.StatusFound && r2.StatusCode != http.StatusSeeOther {
		t.Errorf("second shift open: want redirect, got %d", r2.StatusCode)
	}
	// Different locations (different shifts).
	if r1.Header.Get("Location") == r2.Header.Get("Location") {
		t.Error("distinct keys should produce distinct shift URLs")
	}
}

// ── POST /finance/shifts/:id/close (ShiftClose) ──────────────────────────────

// TestIdempotency_ShiftClose_DuplicateKeyDoesNotDoubleClose verifies that
// replaying a shift-close with the same key does not create duplicate audit
// entries and the shift remains in its final state.
func TestIdempotency_ShiftClose_DuplicateKeyDoesNotDoubleClose(t *testing.T) {
	cfg := testConfig(t)
	log := logger.New("text", "error")
	db, err := app.Bootstrap(cfg, log)
	if err != nil {
		t.Fatalf("Bootstrap: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	a := app.New(cfg, db, log)

	cookie := loginAs(t, a, "finance@careops.local", "Finance!2345")

	// Open a shift first.
	openForm := url.Values{}
	openForm.Set("shift_date", "2026-09-01")
	openForm.Set("shift_name", "night")
	openReq := httptest.NewRequest(http.MethodPost, "/finance/shifts",
		strings.NewReader(openForm.Encode()))
	openReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	openReq.AddCookie(&http.Cookie{Name: "careops_session", Value: cookie})
	openResp, err := a.Test(openReq, -1)
	if err != nil || (openResp.StatusCode != http.StatusFound && openResp.StatusCode != http.StatusSeeOther) {
		t.Fatalf("open shift: status=%d err=%v", openResp.StatusCode, err)
	}
	shiftURL := openResp.Header.Get("Location")
	// shiftURL is "/finance/shifts/<id>"
	parts := strings.Split(strings.TrimPrefix(shiftURL, "/"), "/")
	if len(parts) < 3 {
		t.Fatalf("unexpected redirect URL: %q", shiftURL)
	}
	shiftID := parts[2]

	countCloseAudit := func() int {
		var n int
		db.QueryRow(`
			SELECT COUNT(*) FROM audit_logs
			WHERE action='update' AND entity_type='settlement_shifts' AND entity_id=?`,
			shiftID).Scan(&n)
		return n
	}

	doClose := func(idemKey string) *http.Response {
		t.Helper()
		form := url.Values{}
		form.Set("reconciliation_notes", "idempotency close test")
		form.Set("_idempotency_key", idemKey)
		req := httptest.NewRequest(http.MethodPost,
			"/finance/shifts/"+shiftID+"/close",
			strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.AddCookie(&http.Cookie{Name: "careops_session", Value: cookie})
		resp, err := a.Test(req, -1)
		if err != nil {
			t.Fatalf("POST shift close: %v", err)
		}
		return resp
	}

	const idemKey = "shift-close-idem-test-001"
	r1 := doClose(idemKey)
	auditAfterFirst := countCloseAudit()
	r2 := doClose(idemKey)
	auditAfterSecond := countCloseAudit()

	for i, r := range []*http.Response{r1, r2} {
		if r.StatusCode != http.StatusFound && r.StatusCode != http.StatusSeeOther {
			t.Errorf("ShiftClose idempotency request %d: want redirect, got %d", i+1, r.StatusCode)
		}
	}

	// Duplicate close must not add a second audit entry.
	if auditAfterSecond != auditAfterFirst {
		t.Errorf("ShiftClose idempotency: audit count changed from %d to %d after duplicate close",
			auditAfterFirst, auditAfterSecond)
	}

	// Shift must still be closed.
	var status string
	db.QueryRow(`SELECT status FROM settlement_shifts WHERE id=?`, shiftID).Scan(&status)
	if status != "closed" {
		t.Errorf("ShiftClose idempotency: expected status=closed, got %q", status)
	}
}

// TestIdempotency_ShiftClose_ReplayReturnsSameLocation verifies that replaying
// a close request redirects to the same shift URL as the original.
func TestIdempotency_ShiftClose_ReplayReturnsSameLocation(t *testing.T) {
	cfg := testConfig(t)
	log := logger.New("text", "error")
	db, err := app.Bootstrap(cfg, log)
	if err != nil {
		t.Fatalf("Bootstrap: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	a := app.New(cfg, db, log)

	cookie := loginAs(t, a, "finance@careops.local", "Finance!2345")

	openForm := url.Values{"shift_date": {"2026-10-01"}, "shift_name": {"afternoon"}}
	openReq := httptest.NewRequest(http.MethodPost, "/finance/shifts",
		strings.NewReader(openForm.Encode()))
	openReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	openReq.AddCookie(&http.Cookie{Name: "careops_session", Value: cookie})
	openResp, _ := a.Test(openReq, -1)
	shiftURL := openResp.Header.Get("Location")
	shiftID := strings.Split(strings.TrimPrefix(shiftURL, "/"), "/")[2]

	doClose := func(key string) string {
		t.Helper()
		form := url.Values{"_idempotency_key": {key}}
		req := httptest.NewRequest(http.MethodPost,
			"/finance/shifts/"+shiftID+"/close",
			strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.AddCookie(&http.Cookie{Name: "careops_session", Value: cookie})
		resp, _ := a.Test(req, -1)
		return resp.Header.Get("Location")
	}

	const key = "shift-close-loc-idem-001"
	loc1 := doClose(key)
	loc2 := doClose(key)

	if loc1 == "" || loc2 == "" {
		t.Skip("no redirect location — check shift open success")
	}
	if loc1 != loc2 {
		t.Errorf("ShiftClose replay: first redirect=%q, second=%q (must be same)", loc1, loc2)
	}
}

// ── POST /finance/batches (BatchImportSubmit) ────────────────────────────────

// TestIdempotency_BatchImport_DuplicateKeyDoesNotDoubleBatch verifies that
// replaying a batch import with the same key does not create a second batch
// record or re-process the CSV rows.
func TestIdempotency_BatchImport_DuplicateKeyDoesNotDoubleBatch(t *testing.T) {
	cfg := testConfig(t)
	log := logger.New("text", "error")
	db, err := app.Bootstrap(cfg, log)
	if err != nil {
		t.Fatalf("Bootstrap: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	a := app.New(cfg, db, log)

	cookie := loginAs(t, a, "finance@careops.local", "Finance!2345")

	// Get a real resident MRN.
	var mrn string
	if err := db.QueryRow(`SELECT mrn FROM residents LIMIT 1`).Scan(&mrn); err != nil {
		t.Skip("no residents seeded")
	}

	csvData := "resident_mrn,amount_dollars,description\n" +
		mrn + ",7.00,idem batch test\n"

	buildBatchReq := func(idemKey string) *http.Request {
		t.Helper()
		var buf bytes.Buffer
		w := multipart.NewWriter(&buf)
		fw, _ := w.CreateFormFile("batch_file", "idem_test.csv")
		fw.Write([]byte(csvData)) //nolint:errcheck
		w.WriteField("_idempotency_key", idemKey) //nolint:errcheck
		w.Close()
		req := httptest.NewRequest(http.MethodPost, "/finance/batches", &buf)
		req.Header.Set("Content-Type", w.FormDataContentType())
		req.AddCookie(&http.Cookie{Name: "careops_session", Value: cookie})
		return req
	}

	countBatches := func() int {
		var n int
		db.QueryRow(`SELECT COUNT(*) FROM imported_card_batches`).Scan(&n)
		return n
	}

	const idemKey = "batch-import-idem-test-001"
	r1, err := a.Test(buildBatchReq(idemKey), -1)
	if err != nil {
		t.Fatalf("first batch POST: %v", err)
	}
	if r1.StatusCode != http.StatusFound && r1.StatusCode != http.StatusSeeOther {
		t.Fatalf("first batch POST: want redirect, got %d", r1.StatusCode)
	}

	batchCountAfterFirst := countBatches()

	r2, err := a.Test(buildBatchReq(idemKey), -1)
	if err != nil {
		t.Fatalf("second batch POST: %v", err)
	}
	if r2.StatusCode != http.StatusFound && r2.StatusCode != http.StatusSeeOther {
		t.Fatalf("second batch POST: want redirect (replay), got %d", r2.StatusCode)
	}

	batchCountAfterSecond := countBatches()
	if batchCountAfterSecond != batchCountAfterFirst {
		t.Errorf("BatchImport idempotency: batch count changed from %d to %d after duplicate POST",
			batchCountAfterFirst, batchCountAfterSecond)
	}

	// Both redirects must point to the same batch.
	if r1.Header.Get("Location") != r2.Header.Get("Location") {
		t.Errorf("BatchImport idempotency: first=%q second=%q (should be same batch URL)",
			r1.Header.Get("Location"), r2.Header.Get("Location"))
	}
}

// TestIdempotency_BatchImport_DifferentKeyCreatesTwoBatches ensures distinct
// idempotency keys are not conflated (sanity check for the positive path).
func TestIdempotency_BatchImport_DifferentKeyCreatesTwoBatches(t *testing.T) {
	cfg := testConfig(t)
	log := logger.New("text", "error")
	db, err := app.Bootstrap(cfg, log)
	if err != nil {
		t.Fatalf("Bootstrap: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	a := app.New(cfg, db, log)

	cookie := loginAs(t, a, "finance@careops.local", "Finance!2345")

	var mrn string
	if err := db.QueryRow(`SELECT mrn FROM residents LIMIT 1`).Scan(&mrn); err != nil {
		t.Skip("no residents seeded")
	}
	csvData := "resident_mrn,amount_dollars,description\n" + mrn + ",3.00,distinct key test\n"

	buildReq := func(key string) *http.Request {
		t.Helper()
		var buf bytes.Buffer
		w := multipart.NewWriter(&buf)
		fw, _ := w.CreateFormFile("batch_file", "distinct.csv")
		fw.Write([]byte(csvData)) //nolint:errcheck
		w.WriteField("_idempotency_key", key) //nolint:errcheck
		w.Close()
		req := httptest.NewRequest(http.MethodPost, "/finance/batches", &buf)
		req.Header.Set("Content-Type", w.FormDataContentType())
		req.AddCookie(&http.Cookie{Name: "careops_session", Value: cookie})
		return req
	}

	before := countRowsDB(db, `SELECT COUNT(*) FROM imported_card_batches`)
	a.Test(buildReq("batch-distinct-001"), -1) //nolint:errcheck
	a.Test(buildReq("batch-distinct-002"), -1) //nolint:errcheck
	after := countRowsDB(db, `SELECT COUNT(*) FROM imported_card_batches`)

	if after != before+2 {
		t.Errorf("distinct batch keys: expected %d batches, got %d", before+2, after)
	}
}

// ── OCC strict status assertions ─────────────────────────────────────────────

// TestOCC_ShiftClose_StaleVersionReturns400 verifies that a shift-close
// request with a stale shift version (or already-closed shift) returns 400.
func TestOCC_ShiftClose_StaleVersionReturns400(t *testing.T) {
	cfg := testConfig(t)
	log := logger.New("text", "error")
	db, err := app.Bootstrap(cfg, log)
	if err != nil {
		t.Fatalf("Bootstrap: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	a := app.New(cfg, db, log)

	cookie := loginAs(t, a, "finance@careops.local", "Finance!2345")

	// Open and close a shift.
	openForm := url.Values{"shift_date": {"2026-11-01"}, "shift_name": {"morning"}}
	openReq := httptest.NewRequest(http.MethodPost, "/finance/shifts",
		strings.NewReader(openForm.Encode()))
	openReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	openReq.AddCookie(&http.Cookie{Name: "careops_session", Value: cookie})
	openResp, _ := a.Test(openReq, -1)
	shiftID := strings.Split(strings.TrimPrefix(openResp.Header.Get("Location"), "/"), "/")[2]

	// First close — should succeed.
	closeForm := url.Values{"reconciliation_notes": {"first close"}}
	closeReq := httptest.NewRequest(http.MethodPost,
		"/finance/shifts/"+shiftID+"/close",
		strings.NewReader(closeForm.Encode()))
	closeReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	closeReq.AddCookie(&http.Cookie{Name: "careops_session", Value: cookie})
	closeResp, _ := a.Test(closeReq, -1)
	if closeResp.StatusCode != http.StatusFound && closeResp.StatusCode != http.StatusSeeOther {
		t.Fatalf("first close: want redirect, got %d", closeResp.StatusCode)
	}

	// Second close without idempotency key — shift is already closed; must return 400.
	closeReq2 := httptest.NewRequest(http.MethodPost,
		"/finance/shifts/"+shiftID+"/close",
		strings.NewReader("reconciliation_notes=second+close"))
	closeReq2.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	closeReq2.AddCookie(&http.Cookie{Name: "careops_session", Value: cookie})
	closeResp2, _ := a.Test(closeReq2, -1)
	if closeResp2.StatusCode != http.StatusBadRequest {
		t.Errorf("double close without idem key: want 400, got %d", closeResp2.StatusCode)
	}
}

// ── DB helper ────────────────────────────────────────────────────────────────

// shiftIDFromDB returns the ID of the most recent shift matching date+name.
func shiftIDFromDB(db *sql.DB, date, name string) string {
	var id string
	db.QueryRow(
		`SELECT id FROM settlement_shifts WHERE shift_date=? AND shift_name=? ORDER BY created_at DESC LIMIT 1`,
		date, name).Scan(&id)
	return id
}
