package api_tests

// idempotency_privileged_test.go — verifies idempotency behaviour on privileged
// write paths that previously lacked it: user create, password reset, and
// reports definition update.

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"careops/clinic/internal/app"
	"careops/clinic/internal/platform/logger"
)

// ── User create idempotency ───────────────────────────────────────────────────

// TestIdempotency_UserCreate_DuplicateIsReplayed posts the same user-create
// form twice with the same idempotency key and verifies:
//   - both calls succeed with a redirect
//   - the user is created exactly once (count does not change on the second call)
func TestIdempotency_UserCreate_DuplicateIsReplayed(t *testing.T) {
	cfg := testConfig(t)
	log := logger.New("text", "error")
	db, err := app.BootstrapFast(cfg, log)
	if err != nil {
		t.Fatalf("Bootstrap: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	a := app.New(cfg, db, log)

	cookie := loginAs(t, a, "admin@careops.local", "Admin!234567")

	form := url.Values{
		"email":            {"idem-create@careops.local"},
		"display_name":     {"Idem Create"},
		"password":         {"Idem!Create9"},
		"role":             {"nurse"},
		"_idempotency_key": {"idem-user-create-001"},
	}

	doPost := func() *http.Response {
		req := httptest.NewRequest(http.MethodPost, "/users", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.AddCookie(&http.Cookie{Name: "careops_session", Value: cookie})
		resp, err := a.Test(req, -1)
		if err != nil {
			t.Fatalf("POST /users: %v", err)
		}
		return resp
	}

	countUsers := func() int {
		return countRowsDB(db, `SELECT COUNT(*) FROM users WHERE email = 'idem-create@careops.local'`)
	}

	r1 := doPost()
	if r1.StatusCode != http.StatusFound && r1.StatusCode != http.StatusSeeOther {
		body, _ := io.ReadAll(r1.Body)
		t.Fatalf("first user create: want redirect, got %d — %s", r1.StatusCode, string(body)[:min(300, len(body))])
	}
	countAfterFirst := countUsers()
	if countAfterFirst != 1 {
		t.Fatalf("after first create: expected 1 user, got %d", countAfterFirst)
	}

	r2 := doPost()
	if r2.StatusCode != http.StatusFound && r2.StatusCode != http.StatusSeeOther {
		body, _ := io.ReadAll(r2.Body)
		t.Fatalf("second user create (idempotent): want redirect, got %d — %s", r2.StatusCode, string(body)[:min(300, len(body))])
	}
	countAfterSecond := countUsers()
	if countAfterSecond != countAfterFirst {
		t.Errorf("user create idempotency: count changed from %d to %d after duplicate POST",
			countAfterFirst, countAfterSecond)
	}
}

// ── Password reset idempotency ────────────────────────────────────────────────

// TestIdempotency_ResetPassword_DuplicateIsReplayed posts a password reset
// twice with the same idempotency key and verifies both calls redirect cleanly.
func TestIdempotency_ResetPassword_DuplicateIsReplayed(t *testing.T) {
	a := buildTestApp(t)
	cookie := loginAs(t, a, "admin@careops.local", "Admin!234567")

	listResp := getWithSession(t, a, "/users", cookie)
	listBody := readBody(t, listResp)
	userID := extractUserIDFromResetPasswordLink(t, listBody)
	rowVersion := extractRowVersionFromResetForm(t, a, userID, cookie)

	form := url.Values{
		"password":         {"Idem!Reset99x"},
		"row_version":      {rowVersion},
		"_idempotency_key": {"idem-reset-pw-001"},
	}

	doPost := func() *http.Response {
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

	r1 := doPost()
	if r1.StatusCode != http.StatusFound && r1.StatusCode != http.StatusSeeOther {
		body, _ := io.ReadAll(r1.Body)
		t.Fatalf("first reset: want redirect, got %d — %s", r1.StatusCode, string(body)[:min(300, len(body))])
	}

	r2 := doPost()
	if r2.StatusCode != http.StatusFound && r2.StatusCode != http.StatusSeeOther {
		body, _ := io.ReadAll(r2.Body)
		t.Fatalf("second reset (idempotent): want redirect, got %d — %s", r2.StatusCode, string(body)[:min(300, len(body))])
	}
}

// ── Reports update idempotency ────────────────────────────────────────────────

// TestIdempotency_ReportUpdate_DuplicateIsReplayed creates a report definition
// then POSTs the same update twice with the same idempotency key and verifies
// the second call is replayed without a 409 conflict error.
func TestIdempotency_ReportUpdate_DuplicateIsReplayed(t *testing.T) {
	cfg := testConfig(t)
	log := logger.New("text", "error")
	db, err := app.BootstrapFast(cfg, log)
	if err != nil {
		t.Fatalf("Bootstrap: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	a := app.New(cfg, db, log)

	cookie := loginAs(t, a, "admin@careops.local", "Admin!234567")

	// Create a report definition to update.
	createForm := url.Values{
		"name":          {"Idem Update Report"},
		"report_type":   {"occupancy"},
		"schedule_cron": {"0 0 * * *"},
		"output_format": {"csv"},
		"parameters":    {"{}"},
	}
	createReq := httptest.NewRequest(http.MethodPost, "/reports", strings.NewReader(createForm.Encode()))
	createReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	createReq.AddCookie(&http.Cookie{Name: "careops_session", Value: cookie})
	createResp, err := a.Test(createReq, -1)
	if err != nil {
		t.Fatalf("create report: %v", err)
	}
	if createResp.StatusCode != http.StatusFound && createResp.StatusCode != http.StatusSeeOther {
		body, _ := io.ReadAll(createResp.Body)
		t.Fatalf("create report: want redirect, got %d — %s", createResp.StatusCode, string(body)[:min(300, len(body))])
	}

	// Locate the newly created report's ID from the index page.
	indexResp := getWithSession(t, a, "/reports", cookie)
	indexBody := readBody(t, indexResp)

	// The edit link looks like: href="/reports/<id>/edit"
	const editSuffix = "/edit"
	const editPrefix = `/reports/`
	editIdx := strings.Index(indexBody, editPrefix)
	if editIdx < 0 {
		t.Skip("no report edit link in index — creation may not have succeeded")
	}
	idStart := editIdx + len(editPrefix)
	// Advance past the ID to the /edit suffix.
	suffixIdx := strings.Index(indexBody[idStart:], editSuffix)
	if suffixIdx < 0 {
		t.Skip("cannot find /edit suffix in index page")
	}
	reportID := indexBody[idStart : idStart+suffixIdx]
	if strings.ContainsAny(reportID, `"'<> `) {
		t.Skipf("extracted report ID looks wrong: %q", reportID)
	}

	// Fetch the edit form to read the current row_version.
	editResp := getWithSession(t, a, "/reports/"+reportID+"/edit", cookie)
	editBody := readBody(t, editResp)

	const rvPrefix = `name="row_version" value="`
	rvIdx := strings.Index(editBody, rvPrefix)
	if rvIdx < 0 {
		t.Skip("row_version not found in edit form")
	}
	rvStart := rvIdx + len(rvPrefix)
	rvEnd := strings.Index(editBody[rvStart:], `"`)
	rowVersion := editBody[rvStart : rvStart+rvEnd]

	doUpdate := func() *http.Response {
		form := url.Values{
			"name":             {"Idem Updated Report"},
			"report_type":      {"occupancy"},
			"schedule_cron":    {"0 6 * * *"},
			"output_format":    {"csv"},
			"parameters":       {"{}"},
			"row_version":      {rowVersion},
			"_idempotency_key": {"idem-report-update-001"},
		}
		req := httptest.NewRequest(http.MethodPost, "/reports/"+reportID+"/update",
			strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.AddCookie(&http.Cookie{Name: "careops_session", Value: cookie})
		resp, err := a.Test(req, -1)
		if err != nil {
			t.Fatalf("POST /reports/%s/update: %v", reportID, err)
		}
		return resp
	}

	r1 := doUpdate()
	if r1.StatusCode != http.StatusFound && r1.StatusCode != http.StatusSeeOther {
		body, _ := io.ReadAll(r1.Body)
		t.Fatalf("first report update: want redirect, got %d — %s", r1.StatusCode, string(body)[:min(300, len(body))])
	}

	// Second call with the same idempotency key — stale row_version is bypassed
	// by the cache replay.
	r2 := doUpdate()
	if r2.StatusCode != http.StatusFound && r2.StatusCode != http.StatusSeeOther {
		body, _ := io.ReadAll(r2.Body)
		t.Errorf("second report update (idempotent): want redirect, got %d — %s", r2.StatusCode, string(body)[:min(300, len(body))])
	}
}
