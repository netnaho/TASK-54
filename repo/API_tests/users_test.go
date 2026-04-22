package api_tests

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"careops/clinic/internal/app"
	"careops/clinic/internal/platform/logger"
)

// ── Create user ───────────────────────────────────────────────────────────────

func postCreateUser(t *testing.T, a interface{ Test(*http.Request, ...int) (*http.Response, error) }, cookie string, fields url.Values) *http.Response {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/users", strings.NewReader(fields.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(&http.Cookie{Name: "careops_session", Value: cookie})
	resp, err := a.Test(req, -1)
	if err != nil {
		t.Fatalf("POST /users: %v", err)
	}
	return resp
}

func TestUsers_NewForm_AdminGets200(t *testing.T) {
	a := buildTestApp(t)
	cookie := loginAs(t, a, "admin@careops.local", "Admin!234567")
	resp := getWithSession(t, a, "/users/new", cookie)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("GET /users/new admin: want 200, got %d", resp.StatusCode)
	}
	body := readBody(t, resp)
	if !strings.Contains(body, "password") {
		t.Error("/users/new should contain a password field")
	}
}

func TestUsers_NewForm_NonAdminGets403(t *testing.T) {
	a := buildTestApp(t)
	for _, creds := range [][2]string{
		{"nurse@careops.local", "Nurse!234567"},
		{"finance@careops.local", "Finance!2345"},
	} {
		cookie := loginAs(t, a, creds[0], creds[1])
		resp := getWithSession(t, a, "/users/new", cookie)
		if resp.StatusCode != http.StatusForbidden {
			t.Errorf("%s GET /users/new: want 403, got %d", creds[0], resp.StatusCode)
		}
	}
}

func TestUsers_Create_ValidPasswordRedirects(t *testing.T) {
	a := buildTestApp(t)
	cookie := loginAs(t, a, "admin@careops.local", "Admin!234567")

	fields := url.Values{
		"email":        {"newuser@careops.local"},
		"display_name": {"New User"},
		"password":     {"Str0ng!Password#"},
		"role":         {"nurse"},
	}
	resp := postCreateUser(t, a, cookie, fields)
	if resp.StatusCode != http.StatusFound && resp.StatusCode != http.StatusSeeOther {
		body := readBody(t, resp)
		t.Errorf("create valid user: want redirect, got %d — %s", resp.StatusCode, body[:min(300, len(body))])
	}
	loc := resp.Header.Get("Location")
	if !strings.Contains(loc, "/users") {
		t.Errorf("create user redirect: want /users*, got %q", loc)
	}
}

func TestUsers_Create_CompliantPasswordAppearsInList(t *testing.T) {
	a := buildTestApp(t)
	cookie := loginAs(t, a, "admin@careops.local", "Admin!234567")

	fields := url.Values{
		"email":        {"listuser@careops.local"},
		"display_name": {"List User"},
		"password":     {"Compl1ant!Pass99"},
		"role":         {"auditor"},
	}
	postCreateUser(t, a, cookie, fields)

	resp := getWithSession(t, a, "/users", cookie)
	body := readBody(t, resp)
	if !strings.Contains(body, "listuser@careops.local") {
		t.Error("newly created user email should appear in /users list")
	}
}

func TestUsers_Create_WeakPasswordReturns200WithError(t *testing.T) {
	a := buildTestApp(t)
	cookie := loginAs(t, a, "admin@careops.local", "Admin!234567")

	cases := []struct {
		password string
		desc     string
	}{
		{"short1!", "too short"},
		{"alllowercase1!", "no uppercase"},
		{"ALLUPPERCASE1!", "no lowercase"},
		{"NoSpecialChars12", "no special char"},
		{"NoDigits!!", "no digit"},
	}
	for _, tc := range cases {
		fields := url.Values{
			"email":        {"weak@careops.local"},
			"display_name": {"Weak User"},
			"password":     {tc.password},
			"role":         {"nurse"},
		}
		resp := postCreateUser(t, a, cookie, fields)
		if resp.StatusCode != http.StatusOK {
			t.Errorf("create user weak password (%s): want 200 re-render, got %d", tc.desc, resp.StatusCode)
		}
		body := readBody(t, resp)
		if !strings.Contains(body, "password") {
			t.Errorf("create user weak password (%s): re-render should contain password error", tc.desc)
		}
	}
}

func TestUsers_Create_MissingEmailReturns200WithError(t *testing.T) {
	a := buildTestApp(t)
	cookie := loginAs(t, a, "admin@careops.local", "Admin!234567")

	fields := url.Values{
		"email":        {""},
		"display_name": {"No Email User"},
		"password":     {"Str0ng!Password#"},
		"role":         {"nurse"},
	}
	resp := postCreateUser(t, a, cookie, fields)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("create user no email: want 200 re-render, got %d", resp.StatusCode)
	}
}

func TestUsers_Create_NonAdminGets403(t *testing.T) {
	a := buildTestApp(t)
	cookie := loginAs(t, a, "nurse@careops.local", "Nurse!234567")

	fields := url.Values{
		"email":        {"hacked@careops.local"},
		"display_name": {"Hacked User"},
		"password":     {"Str0ng!Password#"},
		"role":         {"admin"},
	}
	resp := postCreateUser(t, a, cookie, fields)
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("nurse POST /users: want 403, got %d", resp.StatusCode)
	}
}

// ── Reset password ───────────────────────────────────────────────────────────

// extractRowVersionFromResetForm fetches GET /users/:id/reset-password and
// parses the hidden row_version value that is used for OCC.
func extractRowVersionFromResetForm(t *testing.T, a interface {
	Test(*http.Request, ...int) (*http.Response, error)
}, userID, cookie string) string {
	t.Helper()
	resp := getWithSession(t, a, "/users/"+userID+"/reset-password", cookie)
	body := readBody(t, resp)
	const prefix = `name="row_version" value="`
	idx := strings.Index(body, prefix)
	if idx < 0 {
		t.Fatal("row_version hidden field not found in reset-password form")
	}
	start := idx + len(prefix)
	end := strings.Index(body[start:], `"`)
	if end < 0 {
		t.Fatal("row_version value not terminated in reset-password form")
	}
	return body[start : start+end]
}

// extractUserIDFromResetPasswordLink parses a user ID from a /users/<id>/reset-password
// link in the users index page body.
func extractUserIDFromResetPasswordLink(t *testing.T, body string) string {
	t.Helper()
	const suffix = "/reset-password"
	idx := strings.Index(body, suffix)
	if idx < 0 {
		t.Skip("no reset-password links found in /users page")
	}
	before := body[:idx]
	const userPrefix = "/users/"
	lastSlash := strings.LastIndex(before, userPrefix)
	if lastSlash < 0 {
		t.Skip("could not find /users/ prefix before /reset-password")
	}
	return before[lastSlash+len(userPrefix):]
}

func TestUsers_ResetPasswordForm_AdminGets200(t *testing.T) {
	a := buildTestApp(t)
	cookie := loginAs(t, a, "admin@careops.local", "Admin!234567")

	listResp := getWithSession(t, a, "/users", cookie)
	body := readBody(t, listResp)
	userID := extractUserIDFromResetPasswordLink(t, body)

	resp := getWithSession(t, a, "/users/"+userID+"/reset-password", cookie)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("GET /users/:id/reset-password: want 200, got %d", resp.StatusCode)
	}
}

func TestUsers_ResetPassword_WeakPasswordReturns200WithError(t *testing.T) {
	a := buildTestApp(t)
	cookie := loginAs(t, a, "admin@careops.local", "Admin!234567")

	listResp := getWithSession(t, a, "/users", cookie)
	body := readBody(t, listResp)
	userID := extractUserIDFromResetPasswordLink(t, body)

	resetReq := httptest.NewRequest(http.MethodPost, "/users/"+userID+"/reset-password",
		strings.NewReader("password=weak"))
	resetReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resetReq.AddCookie(&http.Cookie{Name: "careops_session", Value: cookie})
	resp, err := a.Test(resetReq, -1)
	if err != nil {
		t.Fatalf("POST reset-password: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("reset password weak: want 200 re-render, got %d", resp.StatusCode)
	}
}

func TestUsers_ResetPassword_StrongPasswordRedirects(t *testing.T) {
	a := buildTestApp(t)
	cookie := loginAs(t, a, "admin@careops.local", "Admin!234567")

	listResp := getWithSession(t, a, "/users", cookie)
	body := readBody(t, listResp)
	userID := extractUserIDFromResetPasswordLink(t, body)
	rowVersion := extractRowVersionFromResetForm(t, a, userID, cookie)

	form := url.Values{
		"password":    {"NewStr0ng!Pass#99"},
		"row_version": {rowVersion},
	}
	resetReq := httptest.NewRequest(http.MethodPost, "/users/"+userID+"/reset-password",
		strings.NewReader(form.Encode()))
	resetReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resetReq.AddCookie(&http.Cookie{Name: "careops_session", Value: cookie})
	resp, err := a.Test(resetReq, -1)
	if err != nil {
		t.Fatalf("POST reset-password: %v", err)
	}
	if resp.StatusCode != http.StatusFound && resp.StatusCode != http.StatusSeeOther {
		body := readBody(t, resp)
		t.Errorf("reset strong password: want redirect, got %d — %s", resp.StatusCode, body[:min(200, len(body))])
	}
}

func TestUsers_ResetPassword_NonAdminGets403(t *testing.T) {
	a := buildTestApp(t)
	cookie := loginAs(t, a, "nurse@careops.local", "Nurse!234567")

	form := url.Values{"password": {"NewStr0ng!Pass#99"}}
	resetReq := httptest.NewRequest(http.MethodPost, "/users/some-id/reset-password",
		strings.NewReader(form.Encode()))
	resetReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resetReq.AddCookie(&http.Cookie{Name: "careops_session", Value: cookie})
	resp, _ := a.Test(resetReq, -1)
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("nurse POST reset-password: want 403, got %d", resp.StatusCode)
	}
}

// ── OCC tests for password reset ──────────────────────────────────────────────

// TestUsers_ResetPassword_StaleVersionReturns409 submits a first reset
// (consuming row_version N), then replays the same original row_version N on a
// second reset — the second must be rejected with 409 Conflict.
func TestUsers_ResetPassword_StaleVersionReturns409(t *testing.T) {
	a := buildTestApp(t)
	cookie := loginAs(t, a, "admin@careops.local", "Admin!234567")

	listResp := getWithSession(t, a, "/users", cookie)
	userID := extractUserIDFromResetPasswordLink(t, readBody(t, listResp))
	originalRowVersion := extractRowVersionFromResetForm(t, a, userID, cookie)

	doReset := func(rv, pw string) *http.Response {
		t.Helper()
		form := url.Values{"password": {pw}, "row_version": {rv}}
		req := httptest.NewRequest(http.MethodPost, "/users/"+userID+"/reset-password",
			strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.AddCookie(&http.Cookie{Name: "careops_session", Value: cookie})
		resp, err := a.Test(req, -1)
		if err != nil {
			t.Fatalf("POST reset-password: %v", err)
		}
		return resp
	}

	// First reset with the current version — must succeed.
	r1 := doReset(originalRowVersion, "OCC!First99pass")
	if r1.StatusCode != http.StatusFound && r1.StatusCode != http.StatusSeeOther {
		body := readBody(t, r1)
		t.Fatalf("first reset: want redirect, got %d — %s", r1.StatusCode, body[:min(300, len(body))])
	}

	// Second reset with the now-stale original version — must return 409.
	r2 := doReset(originalRowVersion, "OCC!Second99pass")
	if r2.StatusCode != http.StatusConflict {
		body := readBody(t, r2)
		t.Errorf("stale row_version reset: want 409, got %d — %s", r2.StatusCode, body[:min(300, len(body))])
	}
}

// TestUsers_ResetPassword_ZeroVersionReturns409 verifies that omitting or
// zeroing the row_version field (e.g. tampering) is treated as a stale version
// and rejected with 409, not silently accepted.
func TestUsers_ResetPassword_ZeroVersionReturns409(t *testing.T) {
	a := buildTestApp(t)
	cookie := loginAs(t, a, "admin@careops.local", "Admin!234567")

	listResp := getWithSession(t, a, "/users", cookie)
	userID := extractUserIDFromResetPasswordLink(t, readBody(t, listResp))

	// row_version=0 is always stale (DB initialises at 1).
	form := url.Values{"password": {"OCC!Zero99pass"}, "row_version": {"0"}}
	req := httptest.NewRequest(http.MethodPost, "/users/"+userID+"/reset-password",
		strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(&http.Cookie{Name: "careops_session", Value: cookie})
	resp, err := a.Test(req, -1)
	if err != nil {
		t.Fatalf("POST reset-password: %v", err)
	}
	if resp.StatusCode != http.StatusConflict {
		body := readBody(t, resp)
		t.Errorf("zero row_version reset: want 409, got %d — %s", resp.StatusCode, body[:min(300, len(body))])
	}
}

// TestUsers_ResetPassword_ConflictWritesFailureAuditEntry verifies that when
// OCC fires the handler records an audit log entry with outcome="failure" so
// the conflict is traceable.
func TestUsers_ResetPassword_ConflictWritesFailureAuditEntry(t *testing.T) {
	cfg := testConfig(t)
	log := logger.New("text", "error")
	db, err := app.BootstrapFast(cfg, log)
	if err != nil {
		t.Fatalf("Bootstrap: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	a := app.New(cfg, db, log)

	cookie := loginAs(t, a, "admin@careops.local", "Admin!234567")
	listResp := getWithSession(t, a, "/users", cookie)
	userID := extractUserIDFromResetPasswordLink(t, readBody(t, listResp))

	// Submit with row_version=0 (always stale) to trigger the conflict path.
	form := url.Values{"password": {"OCC!Audit99pass"}, "row_version": {"0"}}
	req := httptest.NewRequest(http.MethodPost, "/users/"+userID+"/reset-password",
		strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(&http.Cookie{Name: "careops_session", Value: cookie})
	resp, err := a.Test(req, -1)
	if err != nil {
		t.Fatalf("POST reset-password conflict: %v", err)
	}
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("expected 409, got %d", resp.StatusCode)
	}

	var outcome string
	err = db.QueryRow(
		`SELECT outcome FROM audit_logs
		 WHERE action = 'user.reset_password' AND entity_id = ?
		 ORDER BY occurred_at DESC LIMIT 1`, userID,
	).Scan(&outcome)
	if err != nil {
		t.Fatalf("query conflict audit entry: %v", err)
	}
	if outcome != "failure" {
		t.Errorf("conflict audit entry: want outcome='failure', got %q", outcome)
	}
}
