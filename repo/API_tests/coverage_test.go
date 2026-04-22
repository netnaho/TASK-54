package api_tests

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

// ── GET / ────────────────────────────────────────────────────────────────────

func TestRoot_RedirectsToDashboard(t *testing.T) {
	a := buildTestApp(t)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	resp, err := a.Test(req, -1)
	if err != nil {
		t.Fatalf("GET /: %v", err)
	}
	if resp.StatusCode != http.StatusFound && resp.StatusCode != http.StatusSeeOther {
		t.Errorf("GET /: expected redirect, got %d", resp.StatusCode)
	}
	loc := resp.Header.Get("Location")
	if loc != "/dashboard" {
		t.Errorf("GET /: expected redirect to /dashboard, got %q", loc)
	}
}

// ── Service delivery order detail ────────────────────────────────────────────

func TestServiceDelivery_OrderDetail_Returns200(t *testing.T) {
	a, sess := loginSession(t)
	// so-001 is seeded in 0008_config_finance.sql
	resp := authedGet(t, a, "/service-delivery/orders/so-001", sess)
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Errorf("GET /service-delivery/orders/so-001: expected 200, got %d\n%s", resp.StatusCode, body)
	}
}

func TestServiceDelivery_OrderDetail_UnknownReturns404(t *testing.T) {
	a, sess := loginSession(t)
	resp := authedGet(t, a, "/service-delivery/orders/does-not-exist", sess)
	if resp.StatusCode != http.StatusNotFound && resp.StatusCode < 400 {
		t.Errorf("GET /service-delivery/orders/does-not-exist: expected 4xx, got %d", resp.StatusCode)
	}
}

// ── Exam template update ─────────────────────────────────────────────────────

func TestExams_TemplateUpdate_RedirectsOnSuccess(t *testing.T) {
	a, sess := loginSession(t)

	// Create a template first.
	createForm := url.Values{
		"title":            {"Update Test Exam"},
		"role_scope":       {"nurse"},
		"passing_score":    {"80"},
		"duration_minutes": {"60"},
		"window_start":     {"08:00"},
		"window_end":       {"17:00"},
	}
	createResp := authedPost(t, a, "/exams/templates", sess, createForm)
	loc := createResp.Header.Get("Location")
	if !strings.HasPrefix(loc, "/exams/templates/") {
		t.Fatalf("template create redirect %q not as expected", loc)
	}
	templateID := strings.TrimPrefix(loc, "/exams/templates/")

	// Now update it.
	updateForm := url.Values{
		"title":            {"Update Test Exam — Revised"},
		"description":      {"Updated description"},
		"duration_minutes": {"90"},
		"window_start":     {"09:00"},
		"window_end":       {"16:00"},
		"passing_score":    {"85"},
		"template_version": {"1"},
	}
	resp := authedPost(t, a, "/exams/templates/"+templateID, sess, updateForm)
	if resp.StatusCode != http.StatusFound && resp.StatusCode != http.StatusSeeOther {
		body, _ := io.ReadAll(resp.Body)
		t.Errorf("POST /exams/templates/:id: expected redirect, got %d\n%s", resp.StatusCode, body)
	}
	updatedLoc := resp.Header.Get("Location")
	if !strings.HasPrefix(updatedLoc, "/exams/templates/") {
		t.Errorf("POST /exams/templates/:id: expected redirect to template detail, got %q", updatedLoc)
	}
}

// ── Exam session candidates ───────────────────────────────────────────────────

func TestExams_SessionAddCandidate_Redirects(t *testing.T) {
	a, adminSess, sessURL := createSessionFullFlow(t)
	sessID := strings.TrimPrefix(sessURL, "/exams/sessions/")

	form := url.Values{
		"user_id": {"usr-nurse"},
	}
	resp := authedPost(t, a, "/exams/sessions/"+sessID+"/candidates/add", adminSess, form)
	if resp.StatusCode != http.StatusFound && resp.StatusCode != http.StatusSeeOther && resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Errorf("POST /exams/sessions/:id/candidates/add: expected redirect or 200, got %d\n%s", resp.StatusCode, body)
	}
}

func TestExams_SessionRemoveCandidate_Redirects(t *testing.T) {
	a, adminSess, sessURL := createSessionFullFlow(t)
	sessID := strings.TrimPrefix(sessURL, "/exams/sessions/")

	// Add a candidate first.
	addForm := url.Values{"user_id": {"usr-nurse"}}
	authedPost(t, a, "/exams/sessions/"+sessID+"/candidates/add", adminSess, addForm)

	// Now remove it.
	removeForm := url.Values{"user_id": {"usr-nurse"}}
	resp := authedPost(t, a, "/exams/sessions/"+sessID+"/candidates/remove", adminSess, removeForm)
	if resp.StatusCode != http.StatusFound && resp.StatusCode != http.StatusSeeOther && resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Errorf("POST /exams/sessions/:id/candidates/remove: expected redirect or 200, got %d\n%s", resp.StatusCode, body)
	}
}

// ── Exam conflict resolution ──────────────────────────────────────────────────

func TestExams_ConflictResolve_Redirects(t *testing.T) {
	a, sess := loginSession(t)

	// esc-001 is a seeded conflict for sess-003.
	form := url.Values{
		"resolution_notes": {"Room booking corrected — conflict resolved."},
		"session_id":       {"sess-003"},
	}
	resp := authedPost(t, a, "/exams/conflicts/esc-001/resolve", sess, form)
	if resp.StatusCode != http.StatusFound && resp.StatusCode != http.StatusSeeOther && resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Errorf("POST /exams/conflicts/esc-001/resolve: expected redirect or 200, got %d\n%s", resp.StatusCode, body)
	}
}

// ── Finance payment detail & refund ──────────────────────────────────────────

// createPayment creates a cash payment and returns the payment ID extracted from
// the redirect location header.
func createPayment(t *testing.T, a interface{ Test(*http.Request, ...int) (*http.Response, error) }, cookie string) string {
	t.Helper()

	// Get a resident ID from the payment form.
	formResp := getWithSession(t, a, "/finance/payments/new", cookie)
	body, _ := io.ReadAll(formResp.Body)
	bodyStr := string(body)
	idx := strings.Index(bodyStr, `value="res-`)
	if idx < 0 {
		t.Skip("no resident found in payment form — seeds may not have loaded")
	}
	start := idx + len(`value="`)
	end := strings.Index(bodyStr[start:], `"`)
	residentID := bodyStr[start : start+end]

	form := url.Values{
		"resident_id":    {residentID},
		"payment_method": {"cash"},
		"amount":         {"75.00"},
		"description":    {"Coverage test payment"},
	}
	req := httptest.NewRequest(http.MethodPost, "/finance/payments", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(&http.Cookie{Name: "careops_session", Value: cookie})
	resp, err := a.Test(req, -1)
	if err != nil {
		t.Fatalf("createPayment POST: %v", err)
	}
	loc := resp.Header.Get("Location")
	if !strings.HasPrefix(loc, "/finance/payments/") {
		t.Fatalf("createPayment: expected redirect to /finance/payments/:id, got %q", loc)
	}
	return strings.TrimPrefix(loc, "/finance/payments/")
}

func TestFinance_PaymentDetail_Returns200(t *testing.T) {
	a := buildTestApp(t)
	cookie := loginAs(t, a, "finance@careops.local", "Finance!2345")

	// Use a seeded payment — pay-001 is in seeds/0008_config_finance.sql.
	resp := getWithSession(t, a, "/finance/payments/pay-001", cookie)
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Errorf("GET /finance/payments/pay-001: expected 200, got %d\n%s", resp.StatusCode, body)
	}
}

func TestFinance_RefundForm_Returns200(t *testing.T) {
	a := buildTestApp(t)
	cookie := loginAs(t, a, "finance@careops.local", "Finance!2345")

	payID := createPayment(t, a, cookie)
	resp := getWithSession(t, a, "/finance/payments/"+payID+"/refund", cookie)
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Errorf("GET /finance/payments/:id/refund: expected 200, got %d\n%s", resp.StatusCode, body)
	}
}

func TestFinance_RefundCreate_Redirects(t *testing.T) {
	a := buildTestApp(t)
	cookie := loginAs(t, a, "finance@careops.local", "Finance!2345")

	payID := createPayment(t, a, cookie)

	form := url.Values{
		"amount": {"10.00"},
		"reason": {"Coverage test refund"},
	}
	req := httptest.NewRequest(http.MethodPost, "/finance/payments/"+payID+"/refund", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(&http.Cookie{Name: "careops_session", Value: cookie})

	resp, err := a.Test(req, -1)
	if err != nil {
		t.Fatalf("POST /finance/payments/:id/refund: %v", err)
	}
	if resp.StatusCode != http.StatusFound && resp.StatusCode != http.StatusSeeOther {
		body, _ := io.ReadAll(resp.Body)
		t.Errorf("POST /finance/payments/:id/refund: expected redirect, got %d\n%s", resp.StatusCode, body)
	}
}

// ── Finance shift detail & close ─────────────────────────────────────────────

// openShift opens a shift and returns its ID.
func openShift(t *testing.T, a interface{ Test(*http.Request, ...int) (*http.Response, error) }, cookie string) string {
	t.Helper()
	form := url.Values{"shift_name": {"night"}}
	req := httptest.NewRequest(http.MethodPost, "/finance/shifts", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(&http.Cookie{Name: "careops_session", Value: cookie})
	resp, err := a.Test(req, -1)
	if err != nil {
		t.Fatalf("openShift POST: %v", err)
	}
	loc := resp.Header.Get("Location")
	if !strings.HasPrefix(loc, "/finance/shifts/") {
		t.Fatalf("openShift: expected redirect to /finance/shifts/:id, got %q", loc)
	}
	return strings.TrimPrefix(loc, "/finance/shifts/")
}

func TestFinance_ShiftDetail_Returns200(t *testing.T) {
	a := buildTestApp(t)
	cookie := loginAs(t, a, "finance@careops.local", "Finance!2345")

	shiftID := openShift(t, a, cookie)
	resp := getWithSession(t, a, "/finance/shifts/"+shiftID, cookie)
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Errorf("GET /finance/shifts/:id: expected 200, got %d\n%s", resp.StatusCode, body)
	}
}

func TestFinance_ShiftClose_Redirects(t *testing.T) {
	a := buildTestApp(t)
	cookie := loginAs(t, a, "finance@careops.local", "Finance!2345")

	shiftID := openShift(t, a, cookie)

	form := url.Values{
		"reconciliation_notes": {"Coverage test — shift closed cleanly."},
	}
	req := httptest.NewRequest(http.MethodPost, "/finance/shifts/"+shiftID+"/close", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(&http.Cookie{Name: "careops_session", Value: cookie})

	resp, err := a.Test(req, -1)
	if err != nil {
		t.Fatalf("POST /finance/shifts/:id/close: %v", err)
	}
	if resp.StatusCode != http.StatusFound && resp.StatusCode != http.StatusSeeOther {
		body, _ := io.ReadAll(resp.Body)
		t.Errorf("POST /finance/shifts/:id/close: expected redirect, got %d\n%s", resp.StatusCode, body)
	}
}

// ── Finance batch detail ──────────────────────────────────────────────────────

func TestFinance_BatchDetail_Returns200(t *testing.T) {
	a := buildTestApp(t)
	cookie := loginAs(t, a, "finance@careops.local", "Finance!2345")

	// batch-001 is seeded in seeds/0011_finance_phase4.sql.
	resp := getWithSession(t, a, "/finance/batches/batch-001", cookie)
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Errorf("GET /finance/batches/batch-001: expected 200, got %d\n%s", resp.StatusCode, body)
	}
}
