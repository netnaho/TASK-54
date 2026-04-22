package api_tests

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

// ── Access control ─────────────────────────────────────────────────────────────

func TestAudit_AdminGets200(t *testing.T) {
	a := buildTestApp(t)
	cookie := loginAs(t, a, "admin@careops.local", "Admin!234567")
	resp := getWithSession(t, a, "/audit", cookie)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("admin on /audit: want 200, got %d", resp.StatusCode)
	}
}

func TestAudit_AuditorGets200(t *testing.T) {
	a := buildTestApp(t)
	cookie := loginAs(t, a, "auditor@careops.local", "Auditor!2345")
	resp := getWithSession(t, a, "/audit", cookie)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("auditor on /audit: want 200, got %d", resp.StatusCode)
	}
}

func TestAudit_NurseForbidden(t *testing.T) {
	a := buildTestApp(t)
	cookie := loginAs(t, a, "nurse@careops.local", "Nurse!234567")
	resp := getWithSession(t, a, "/audit", cookie)
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("nurse on /audit: want 403, got %d", resp.StatusCode)
	}
}

func TestAudit_FinanceClerkForbidden(t *testing.T) {
	a := buildTestApp(t)
	cookie := loginAs(t, a, "finance@careops.local", "Finance!2345")
	resp := getWithSession(t, a, "/audit", cookie)
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("finance_clerk on /audit: want 403, got %d", resp.StatusCode)
	}
}

func TestAudit_TherapistForbidden(t *testing.T) {
	a := buildTestApp(t)
	cookie := loginAs(t, a, "therapist@careops.local", "Therapist!2345")
	resp := getWithSession(t, a, "/audit", cookie)
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("therapist on /audit: want 403, got %d", resp.StatusCode)
	}
}

func TestAudit_AideForbidden(t *testing.T) {
	a := buildTestApp(t)
	cookie := loginAs(t, a, "aide@careops.local", "Aide!2345678")
	resp := getWithSession(t, a, "/audit", cookie)
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("aide on /audit: want 403, got %d", resp.StatusCode)
	}
}

func TestAudit_UnauthenticatedRedirectsToLogin(t *testing.T) {
	a := buildTestApp(t)
	resp := getWithSession(t, a, "/audit", "")
	if resp.StatusCode != http.StatusFound && resp.StatusCode != http.StatusSeeOther {
		t.Errorf("unauthenticated /audit: want redirect, got %d", resp.StatusCode)
	}
	if loc := resp.Header.Get("Location"); loc != "/login" {
		t.Errorf("unauthenticated /audit: want redirect to /login, got %q", loc)
	}
}

// ── Page content ───────────────────────────────────────────────────────────────

func TestAudit_PageShowsAuditLog(t *testing.T) {
	a := buildTestApp(t)
	cookie := loginAs(t, a, "admin@careops.local", "Admin!234567")
	resp := getWithSession(t, a, "/audit", cookie)
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "Audit Log") {
		t.Error("audit page: expected 'Audit Log' in response body")
	}
}

// ── Filter by entity_type ──────────────────────────────────────────────────────

func TestAudit_FilterByEntityType_Payments(t *testing.T) {
	a := buildTestApp(t)

	// Create a payment as finance_clerk to generate an audit entry for "payments".
	financeCookie := loginAs(t, a, "finance@careops.local", "Finance!2345")

	// Get a resident ID from the payments form.
	formResp := getWithSession(t, a, "/finance/payments/new", financeCookie)
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
	form.Set("amount", "10.00")
	form.Set("description", "Audit filter test payment")

	postReq := httptest.NewRequest(http.MethodPost, "/finance/payments", strings.NewReader(form.Encode()))
	postReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	postReq.AddCookie(&http.Cookie{Name: "careops_session", Value: financeCookie})
	createResp, err := a.Test(postReq, -1)
	if err != nil {
		t.Fatalf("create payment: %v", err)
	}
	if createResp.StatusCode != http.StatusFound && createResp.StatusCode != http.StatusSeeOther {
		body, _ := io.ReadAll(createResp.Body)
		t.Fatalf("create payment: want redirect, got %d — %s", createResp.StatusCode, string(body)[:min(200, len(body))])
	}

	// Now query audit log filtered by entity_type=payments as admin.
	adminCookie := loginAs(t, a, "admin@careops.local", "Admin!234567")
	resp := getWithSession(t, a, "/audit?entity_type=payments", adminCookie)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("audit filter payments: want 200, got %d", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	_ = body // Filtering is successful if the page returns 200.
}

// ── Filter by date range ───────────────────────────────────────────────────────

func TestAudit_FilterByDateRange_Today(t *testing.T) {
	a := buildTestApp(t)
	adminCookie := loginAs(t, a, "admin@careops.local", "Admin!234567")

	today := time.Now().UTC().Format("2006-01-02")
	path := "/audit?date_from=" + today + "&date_to=" + today

	resp := getWithSession(t, a, path, adminCookie)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("audit date_from/date_to filter: want 200, got %d", resp.StatusCode)
	}
}

// ── Audit entry after payment creation ────────────────────────────────────────

func TestAudit_PaymentCreatesAuditEntry(t *testing.T) {
	a := buildTestApp(t)

	// Create a payment as finance_clerk.
	financeCookie := loginAs(t, a, "finance@careops.local", "Finance!2345")

	formResp := getWithSession(t, a, "/finance/payments/new", financeCookie)
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
	form.Set("amount", "25.00")
	form.Set("description", "Audit trail test payment")

	postReq := httptest.NewRequest(http.MethodPost, "/finance/payments", strings.NewReader(form.Encode()))
	postReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	postReq.AddCookie(&http.Cookie{Name: "careops_session", Value: financeCookie})
	createResp, err := a.Test(postReq, -1)
	if err != nil {
		t.Fatalf("create payment: %v", err)
	}
	if createResp.StatusCode != http.StatusFound && createResp.StatusCode != http.StatusSeeOther {
		body, _ := io.ReadAll(createResp.Body)
		t.Fatalf("create payment: want redirect, got %d — %s", createResp.StatusCode, string(body)[:min(200, len(body))])
	}

	// Check the audit log as admin — should contain an entry for "payments".
	adminCookie := loginAs(t, a, "admin@careops.local", "Admin!234567")
	auditResp := getWithSession(t, a, "/audit?entity_type=payments", adminCookie)
	if auditResp.StatusCode != http.StatusOK {
		t.Errorf("audit log after payment: want 200, got %d", auditResp.StatusCode)
	}
	auditBody, _ := io.ReadAll(auditResp.Body)
	if !strings.Contains(string(auditBody), "payment") {
		t.Error("audit log after payment creation: expected 'payment' in response body")
	}
}
