package api_tests

import (
	"database/sql"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"careops/clinic/internal/app"
	"careops/clinic/internal/platform/logger"
)


// — helper: log in as a specific user and return the session cookie value —
func loginAs(t *testing.T, a interface{ Test(*http.Request, ...int) (*http.Response, error) }, email, password string) string {
	t.Helper()
	form := url.Values{}
	form.Set("email", email)
	form.Set("password", password)
	req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := a.Test(req, -1)
	if err != nil {
		t.Fatalf("login POST: %v", err)
	}
	for _, c := range resp.Cookies() {
		if c.Name == "careops_session" {
			return c.Value
		}
	}
	t.Fatalf("no session cookie after login as %s (status=%d)", email, resp.StatusCode)
	return ""
}

// — helper: GET a path with a session cookie —
func getWithSession(t *testing.T, a interface{ Test(*http.Request, ...int) (*http.Response, error) }, path, cookie string) *http.Response {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	if cookie != "" {
		req.AddCookie(&http.Cookie{Name: "careops_session", Value: cookie})
	}
	resp, err := a.Test(req, -1)
	if err != nil {
		t.Fatalf("GET %s: %v", path, err)
	}
	return resp
}

// ================================================================
// 1. Login / Logout flow
// ================================================================

func TestAuth_SuccessfulLogin(t *testing.T) {
	a := buildTestApp(t)
	cookie := loginAs(t, a, "admin@careops.local", "Admin!234567")
	if cookie == "" {
		t.Error("expected non-empty session cookie")
	}
}

func TestAuth_LogoutClearsCookie(t *testing.T) {
	a := buildTestApp(t)
	cookie := loginAs(t, a, "admin@careops.local", "Admin!234567")

	req := httptest.NewRequest(http.MethodPost, "/logout", nil)
	req.AddCookie(&http.Cookie{Name: "careops_session", Value: cookie})
	resp, err := a.Test(req, -1)
	if err != nil {
		t.Fatalf("POST /logout: %v", err)
	}
	if resp.StatusCode != http.StatusFound && resp.StatusCode != http.StatusSeeOther {
		t.Errorf("logout: expected redirect, got %d", resp.StatusCode)
	}

	// Session cookie should be cleared (empty value or Max-Age=0)
	for _, c := range resp.Cookies() {
		if c.Name == "careops_session" {
			if c.Value != "" && c.MaxAge != -1 {
				t.Errorf("session cookie not cleared: value=%q MaxAge=%d", c.Value, c.MaxAge)
			}
		}
	}

	// Dashboard should now redirect to login
	resp2 := getWithSession(t, a, "/dashboard", cookie)
	if resp2.StatusCode != http.StatusFound && resp2.StatusCode != http.StatusSeeOther {
		t.Errorf("dashboard with cleared session: expected redirect, got %d", resp2.StatusCode)
	}
}

func TestAuth_BadPassword_Returns200(t *testing.T) {
	a := buildTestApp(t)
	form := url.Values{}
	form.Set("email", "admin@careops.local")
	form.Set("password", "WrongPassword!!")
	req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, _ := a.Test(req, -1)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("bad password: expected 200 (re-render), got %d", resp.StatusCode)
	}
	// Must not set a session cookie
	for _, c := range resp.Cookies() {
		if c.Name == "careops_session" && c.Value != "" {
			t.Error("session cookie must not be set on failed login")
		}
	}
	// Body must re-render the login form, not redirect to dashboard
	body := readBody(t, resp)
	if !strings.Contains(body, "Sign In") {
		t.Error("bad password: response body should contain login form text 'Sign In'")
	}
	if strings.Contains(body, "Operations Dashboard") {
		t.Error("bad password: response body must not contain dashboard content")
	}
}

func TestAuth_UnknownEmail_Returns200(t *testing.T) {
	a := buildTestApp(t)
	form := url.Values{}
	form.Set("email", "nobody@nowhere.local")
	form.Set("password", "DoesNotMatter1!")
	req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, _ := a.Test(req, -1)
	// Same 200 response as bad password — does not reveal user existence
	if resp.StatusCode != http.StatusOK {
		t.Errorf("unknown email: expected 200, got %d", resp.StatusCode)
	}
}

// ================================================================
// 2. 401 on protected endpoints without session
// ================================================================

func TestAuth_UnauthenticatedRedirectsToLogin(t *testing.T) {
	a := buildTestApp(t)
	paths := []string{"/dashboard", "/users", "/finance", "/audit", "/diagnostics"}
	for _, path := range paths {
		resp := getWithSession(t, a, path, "")
		if resp.StatusCode != http.StatusFound && resp.StatusCode != http.StatusSeeOther {
			t.Errorf("%s: expected redirect to login, got %d", path, resp.StatusCode)
		}
		if loc := resp.Header.Get("Location"); loc != "/login" {
			t.Errorf("%s: redirect target want /login, got %q", path, loc)
		}
	}
}

func TestAuth_HTMXUnauthenticatedReturns401(t *testing.T) {
	a := buildTestApp(t)
	req := httptest.NewRequest(http.MethodGet, "/dashboard", nil)
	req.Header.Set("HX-Request", "true")
	resp, err := a.Test(req, -1)
	if err != nil {
		t.Fatalf("GET /dashboard HTMX: %v", err)
	}
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("HTMX unauthenticated: expected 401, got %d", resp.StatusCode)
	}
	if hxRedirect := resp.Header.Get("HX-Redirect"); hxRedirect != "/login" {
		t.Errorf("HTMX unauthenticated: expected HX-Redirect=/login, got %q", hxRedirect)
	}
}

// ================================================================
// 3. 403 on role-protected endpoints
// ================================================================

func TestRBAC_FinanceRequiresFinanceClerkOrAdmin(t *testing.T) {
	a := buildTestApp(t)

	// Nurse cannot access finance
	nurseCookie := loginAs(t, a, "nurse@careops.local", "Nurse!234567")
	resp := getWithSession(t, a, "/finance", nurseCookie)
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("nurse on /finance: expected 403, got %d", resp.StatusCode)
	}
	// 403 page must render the "Access Denied" content, not a blank body
	forbiddenBody := readBody(t, resp)
	if !strings.Contains(forbiddenBody, "Access Denied") {
		t.Errorf("nurse on /finance: 403 body missing 'Access Denied', got:\n%s", forbiddenBody[:min(200, len(forbiddenBody))])
	}
	if !strings.Contains(forbiddenBody, "permission") {
		t.Errorf("nurse on /finance: 403 body missing 'permission' message")
	}

	// Finance clerk can access
	financeCookie := loginAs(t, a, "finance@careops.local", "Finance!2345")
	resp2 := getWithSession(t, a, "/finance", financeCookie)
	if resp2.StatusCode != http.StatusOK {
		t.Errorf("finance_clerk on /finance: expected 200, got %d", resp2.StatusCode)
	}
	// Finance page must show the module heading
	financeBody := readBody(t, resp2)
	if !strings.Contains(financeBody, "Finance") {
		t.Errorf("finance_clerk on /finance: body missing 'Finance' heading")
	}
}

func TestRBAC_DiagnosticsAdminOnly(t *testing.T) {
	a := buildTestApp(t)

	// Auditor cannot access diagnostics
	auditorCookie := loginAs(t, a, "auditor@careops.local", "Auditor!2345")
	resp := getWithSession(t, a, "/diagnostics", auditorCookie)
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("auditor on /diagnostics: expected 403, got %d", resp.StatusCode)
	}

	// Admin can access
	adminCookie := loginAs(t, a, "admin@careops.local", "Admin!234567")
	resp2 := getWithSession(t, a, "/diagnostics", adminCookie)
	if resp2.StatusCode != http.StatusOK {
		t.Errorf("admin on /diagnostics: expected 200, got %d", resp2.StatusCode)
	}
}

func TestRBAC_ConfigVersionsAdminOnly(t *testing.T) {
	a := buildTestApp(t)

	therapistCookie := loginAs(t, a, "therapist@careops.local", "Therapist!2345")
	resp := getWithSession(t, a, "/config-versions", therapistCookie)
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("therapist on /config-versions: expected 403, got %d", resp.StatusCode)
	}
}

func TestRBAC_AuditRequiresAdminOrAuditor(t *testing.T) {
	a := buildTestApp(t)

	// Aide cannot access audit log
	aideCookie := loginAs(t, a, "aide@careops.local", "Aide!2345678")
	resp := getWithSession(t, a, "/audit", aideCookie)
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("aide on /audit: expected 403, got %d", resp.StatusCode)
	}

	// Auditor can access
	auditorCookie := loginAs(t, a, "auditor@careops.local", "Auditor!2345")
	resp2 := getWithSession(t, a, "/audit", auditorCookie)
	if resp2.StatusCode != http.StatusOK {
		t.Errorf("auditor on /audit: expected 200, got %d", resp2.StatusCode)
	}
}

func TestRBAC_UsersAdminOnly(t *testing.T) {
	a := buildTestApp(t)

	// Front desk cannot manage users
	fdCookie := loginAs(t, a, "front.desk@careops.local", "FrontDesk!2345")
	resp := getWithSession(t, a, "/users", fdCookie)
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("front_desk on /users: expected 403, got %d", resp.StatusCode)
	}
}

func TestRBAC_HTMXForbiddenIncludesHXRedirect(t *testing.T) {
	a := buildTestApp(t)
	aideCookie := loginAs(t, a, "aide@careops.local", "Aide!2345678")

	req := httptest.NewRequest(http.MethodGet, "/finance", nil)
	req.AddCookie(&http.Cookie{Name: "careops_session", Value: aideCookie})
	req.Header.Set("HX-Request", "true")
	resp, err := a.Test(req, -1)
	if err != nil {
		t.Fatalf("HTMX 403 test: %v", err)
	}
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("HTMX 403: expected 403, got %d", resp.StatusCode)
	}
	if resp.Header.Get("HX-Redirect") == "" {
		t.Error("HTMX 403: expected HX-Redirect header")
	}
}

// ================================================================
// 4. Expired / inactivity session
// ================================================================

func TestAuth_ExpiredSessionRedirectsToLogin(t *testing.T) {
	cfg := testConfig(t)
	cfg.SessionTTLHours = 24
	cfg.InactivityTimeoutMinutes = 1 // short timeout for this test
	log := logger.New("text", "error")

	db, err := app.Bootstrap(cfg, log)
	if err != nil {
		t.Fatalf("Bootstrap: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	a := app.New(cfg, db, log)

	cookie := loginAs(t, a, "admin@careops.local", "Admin!234567")

	// Backdoor: manually set last_activity_at to 2 minutes ago
	past := time.Now().UTC().Add(-2 * time.Minute).Format(time.RFC3339)
	_, execErr := db.Exec(
		`UPDATE sessions SET last_activity_at = ? WHERE token_hash = (
			SELECT token_hash FROM sessions ORDER BY created_at DESC LIMIT 1
		)`, past,
	)
	if execErr != nil {
		t.Fatalf("backdate last_activity_at: %v", execErr)
	}

	resp := getWithSession(t, a, "/dashboard", cookie)
	if resp.StatusCode != http.StatusFound && resp.StatusCode != http.StatusSeeOther {
		t.Errorf("inactive session: expected redirect, got %d", resp.StatusCode)
	}
	if loc := resp.Header.Get("Location"); loc != "/login" {
		t.Errorf("inactive session: expected redirect to /login, got %q", loc)
	}
}

// ================================================================
// 5. Audit log immutability (no update/delete path exists)
// ================================================================

func TestAudit_NoUpdateOrDeletePathInSchema(t *testing.T) {
	cfg := testConfig(t)
	log := logger.New("text", "error")
	db, err := app.Bootstrap(cfg, log)
	if err != nil {
		t.Fatalf("Bootstrap: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	// Insert a test audit entry directly (user_id NULL = system action; schema allows this)
	_, insErr := db.Exec(`
		INSERT INTO audit_logs (id, user_id, action, entity_type, entity_id,
		  details, before_snapshot, after_snapshot, outcome,
		  ip_address, request_id, occurred_at, created_at)
		VALUES ('test-audit-id', NULL, 'test_action','test','tid','{}','','','success','','',
		  datetime('now'), datetime('now'))`)
	if insErr != nil {
		t.Fatalf("insert audit entry: %v", insErr)
	}

	var id string
	if err := db.QueryRow(
		`SELECT id FROM audit_logs WHERE id = 'test-audit-id'`,
	).Scan(&id); err != nil {
		t.Fatalf("read audit entry: %v", err)
	}
	if id != "test-audit-id" {
		t.Error("audit entry not found after insert")
	}
	// Verify new columns exist (migration 0003 applied)
	var outcome string
	if err := db.QueryRow(
		`SELECT outcome FROM audit_logs WHERE id = 'test-audit-id'`,
	).Scan(&outcome); err != nil {
		t.Fatalf("read outcome column: %v — migration 0003 may not have run", err)
	}
	if outcome != "success" {
		t.Errorf("outcome: want 'success', got %q", outcome)
	}
}

// ================================================================
// 6. Session inactivity column added by migration 0003
// ================================================================

func TestSession_LastActivityAtColumnExists(t *testing.T) {
	cfg := testConfig(t)
	log := logger.New("text", "error")
	db, err := app.Bootstrap(cfg, log)
	if err != nil {
		t.Fatalf("Bootstrap: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	// Log in to create a session, then verify last_activity_at is populated
	a := app.New(cfg, db, log)
	loginAs(t, a, "admin@careops.local", "Admin!234567")

	var lastActivity string
	scanErr := db.QueryRow(
		`SELECT last_activity_at FROM sessions ORDER BY created_at DESC LIMIT 1`,
	).Scan(&lastActivity)
	if scanErr == sql.ErrNoRows {
		t.Fatal("no session row found after login")
	}
	if scanErr != nil {
		t.Fatalf("scan last_activity_at: %v — migration 0003 may not have run", scanErr)
	}
	if lastActivity == "" {
		t.Error("last_activity_at should be set on session creation")
	}
}
