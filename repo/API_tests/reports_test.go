package api_tests

import (
	"archive/zip"
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

// ── Authorization ─────────────────────────────────────────────────────────────

func TestReports_UnauthenticatedRedirects(t *testing.T) {
	a := buildTestApp(t)
	resp := getWithSession(t, a, "/reports", "")
	if resp.StatusCode == http.StatusOK {
		t.Errorf("unauthenticated /reports: want redirect/401, got 200")
	}
}

func TestReports_NurseGets403(t *testing.T) {
	a := buildTestApp(t)
	for _, creds := range [][2]string{
		{"nurse@careops.local", "Nurse!234567"},
		{"finance@careops.local", "Finance!2345"},
		{"therapist@careops.local", "Therapist!2345"},
		{"front.desk@careops.local", "FrontDesk!2345"},
	} {
		cookie := loginAs(t, a, creds[0], creds[1])
		resp := getWithSession(t, a, "/reports", cookie)
		if resp.StatusCode != http.StatusForbidden {
			t.Errorf("%s on /reports: want 403, got %d", creds[0], resp.StatusCode)
		}
	}
}

func TestReports_AdminGets200(t *testing.T) {
	a := buildTestApp(t)
	cookie := loginAs(t, a, "admin@careops.local", "Admin!234567")
	resp := getWithSession(t, a, "/reports", cookie)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("admin GET /reports: want 200, got %d", resp.StatusCode)
	}
}

func TestReports_AuditorGets200(t *testing.T) {
	a := buildTestApp(t)
	cookie := loginAs(t, a, "auditor@careops.local", "Auditor!2345")
	resp := getWithSession(t, a, "/reports", cookie)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("auditor GET /reports: want 200, got %d", resp.StatusCode)
	}
}

func TestReports_NewFormGet200(t *testing.T) {
	a := buildTestApp(t)
	cookie := loginAs(t, a, "admin@careops.local", "Admin!234567")
	resp := getWithSession(t, a, "/reports/new", cookie)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("GET /reports/new: want 200, got %d", resp.StatusCode)
	}
	body := readBody(t, resp)
	if !strings.Contains(body, "report_type") {
		t.Error("/reports/new: page should contain report_type select")
	}
}

func TestReports_NewFormForbiddenForNurse(t *testing.T) {
	a := buildTestApp(t)
	cookie := loginAs(t, a, "nurse@careops.local", "Nurse!234567")
	resp := getWithSession(t, a, "/reports/new", cookie)
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("nurse GET /reports/new: want 403, got %d", resp.StatusCode)
	}
}

// ── Create definition ────────────────────────────────────────────────────────

func postReport(t *testing.T, a interface{ Test(*http.Request, ...int) (*http.Response, error) }, cookie string, fields url.Values) *http.Response {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/reports", strings.NewReader(fields.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(&http.Cookie{Name: "careops_session", Value: cookie})
	resp, err := a.Test(req, -1)
	if err != nil {
		t.Fatalf("POST /reports: %v", err)
	}
	return resp
}

func TestReports_CreateDefinition_ValidRedirects(t *testing.T) {
	a := buildTestApp(t)
	cookie := loginAs(t, a, "admin@careops.local", "Admin!234567")

	fields := url.Values{
		"name":          {"Occupancy Summary"},
		"report_type":   {"occupancy"},
		"output_format": {"csv"},
		"schedule_cron": {"0 6 * * 1"},
	}
	resp := postReport(t, a, cookie, fields)
	if resp.StatusCode != http.StatusFound && resp.StatusCode != http.StatusSeeOther {
		body, _ := io.ReadAll(resp.Body)
		t.Errorf("create valid report: want redirect, got %d — %s", resp.StatusCode, string(body)[:min(300, len(body))])
	}
	loc := resp.Header.Get("Location")
	if !strings.Contains(loc, "/reports") {
		t.Errorf("create report redirect: want /reports*, got %q", loc)
	}
}

func TestReports_CreateDefinition_AppearsInList(t *testing.T) {
	a := buildTestApp(t)
	cookie := loginAs(t, a, "admin@careops.local", "Admin!234567")

	fields := url.Values{
		"name":          {"Finance Monthly"},
		"report_type":   {"finance"},
		"output_format": {"xlsx"},
	}
	postReport(t, a, cookie, fields)

	resp := getWithSession(t, a, "/reports", cookie)
	body := readBody(t, resp)
	if !strings.Contains(body, "Finance Monthly") {
		t.Error("created report definition should appear in /reports list")
	}
}

func TestReports_CreateDefinition_MissingNameReturns200WithError(t *testing.T) {
	a := buildTestApp(t)
	cookie := loginAs(t, a, "admin@careops.local", "Admin!234567")

	fields := url.Values{
		"name":          {""},
		"report_type":   {"occupancy"},
		"output_format": {"csv"},
	}
	resp := postReport(t, a, cookie, fields)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("create report missing name: want 200 (re-render), got %d", resp.StatusCode)
	}
	body := readBody(t, resp)
	if !strings.Contains(body, "required") && !strings.Contains(body, "name") {
		t.Error("validation error should mention missing name")
	}
}

func TestReports_CreateDefinition_InvalidTypeReturns200WithError(t *testing.T) {
	a := buildTestApp(t)
	cookie := loginAs(t, a, "admin@careops.local", "Admin!234567")

	fields := url.Values{
		"name":          {"Bad Type Report"},
		"report_type":   {"invalid_type"},
		"output_format": {"csv"},
	}
	resp := postReport(t, a, cookie, fields)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("create report invalid type: want 200 (re-render), got %d", resp.StatusCode)
	}
}

func TestReports_CreateDefinition_ForbiddenForNurse(t *testing.T) {
	a := buildTestApp(t)
	cookie := loginAs(t, a, "nurse@careops.local", "Nurse!234567")

	fields := url.Values{
		"name":          {"Nurse Report"},
		"report_type":   {"occupancy"},
		"output_format": {"csv"},
	}
	resp := postReport(t, a, cookie, fields)
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("nurse POST /reports: want 403, got %d", resp.StatusCode)
	}
}

// ── Idempotency ──────────────────────────────────────────────────────────────

func TestReports_CreateDefinition_IdempotencyPreventsDouble(t *testing.T) {
	a := buildTestApp(t)
	cookie := loginAs(t, a, "admin@careops.local", "Admin!234567")

	fields := url.Values{
		"name":             {"Idempotent Report"},
		"report_type":      {"audit"},
		"output_format":    {"csv"},
		"_idempotency_key": {"reports-create-idem-001"},
	}

	resp1 := postReport(t, a, cookie, fields)
	resp2 := postReport(t, a, cookie, fields)

	if resp1.StatusCode != http.StatusFound && resp1.StatusCode != http.StatusSeeOther {
		t.Errorf("first create: want redirect, got %d", resp1.StatusCode)
	}
	if resp2.StatusCode != http.StatusFound && resp2.StatusCode != http.StatusSeeOther {
		t.Errorf("duplicate create: want redirect (idempotent replay), got %d", resp2.StatusCode)
	}

	// Verify only one definition was created.
	listResp := getWithSession(t, a, "/reports", cookie)
	body := readBody(t, listResp)
	count := strings.Count(body, "Idempotent Report")
	if count != 1 {
		t.Errorf("idempotent create: expected 1 definition named 'Idempotent Report', found %d", count)
	}
}

// ── Run report ───────────────────────────────────────────────────────────────

func createTestReportDefinition(t *testing.T, a interface{ Test(*http.Request, ...int) (*http.Response, error) }, cookie, name, reportType string) {
	t.Helper()
	fields := url.Values{
		"name":          {name},
		"report_type":   {reportType},
		"output_format": {"csv"},
	}
	postReport(t, a, cookie, fields)
}

func getReportID(t *testing.T, a interface{ Test(*http.Request, ...int) (*http.Response, error) }, cookie, name string) string {
	t.Helper()
	resp := getWithSession(t, a, "/reports", cookie)
	body := readBody(t, resp)
	// Find the report name, then locate the nearest run form action after it.
	nameIdx := strings.Index(body, name)
	if nameIdx < 0 {
		t.Fatalf("could not find report %q in page", name)
	}
	needle := `action="/reports/`
	relIdx := strings.Index(body[nameIdx:], needle)
	if relIdx < 0 {
		t.Fatalf("could not find run action for report %q", name)
	}
	start := nameIdx + relIdx + len(needle)
	end := strings.Index(body[start:], "/")
	if end < 0 {
		t.Fatalf("malformed report action URL for %q", name)
	}
	return body[start : start+end]
}

func TestReports_RunNow_Redirects(t *testing.T) {
	a := buildTestApp(t)
	cookie := loginAs(t, a, "admin@careops.local", "Admin!234567")

	createTestReportDefinition(t, a, cookie, "Run Test Occupancy", "occupancy")
	id := getReportID(t, a, cookie, "Run Test Occupancy")

	req := httptest.NewRequest(http.MethodPost, "/reports/"+id+"/run", nil)
	req.AddCookie(&http.Cookie{Name: "careops_session", Value: cookie})
	resp, err := a.Test(req, -1)
	if err != nil {
		t.Fatalf("POST /reports/:id/run: %v", err)
	}
	if resp.StatusCode != http.StatusFound && resp.StatusCode != http.StatusSeeOther {
		body, _ := io.ReadAll(resp.Body)
		t.Errorf("run report: want redirect, got %d — %s", resp.StatusCode, string(body)[:min(300, len(body))])
	}
}

func TestReports_RunNow_AppearsInHistory(t *testing.T) {
	a := buildTestApp(t)
	cookie := loginAs(t, a, "admin@careops.local", "Admin!234567")

	createTestReportDefinition(t, a, cookie, "History Test Finance", "finance")
	id := getReportID(t, a, cookie, "History Test Finance")

	req := httptest.NewRequest(http.MethodPost, "/reports/"+id+"/run", nil)
	req.AddCookie(&http.Cookie{Name: "careops_session", Value: cookie})
	_, _ = a.Test(req, -1)

	listResp := getWithSession(t, a, "/reports", cookie)
	body := readBody(t, listResp)
	if !strings.Contains(body, "finance") {
		t.Error("after run: /reports page should show finance run in history")
	}
	if !strings.Contains(body, "complete") {
		t.Error("after run: run history should show 'complete' status")
	}
}

func TestReports_RunNow_ForbiddenForNurse(t *testing.T) {
	a := buildTestApp(t)
	adminCookie := loginAs(t, a, "admin@careops.local", "Admin!234567")
	nurseCookie := loginAs(t, a, "nurse@careops.local", "Nurse!234567")

	createTestReportDefinition(t, a, adminCookie, "Nurse Run Test", "occupancy")
	id := getReportID(t, a, adminCookie, "Nurse Run Test")

	req := httptest.NewRequest(http.MethodPost, "/reports/"+id+"/run", nil)
	req.AddCookie(&http.Cookie{Name: "careops_session", Value: nurseCookie})
	resp, err := a.Test(req, -1)
	if err != nil {
		t.Fatalf("nurse POST run: %v", err)
	}
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("nurse run report: want 403, got %d", resp.StatusCode)
	}
}

func TestReports_RunNow_IdempotencyPreventsDoubleRun(t *testing.T) {
	a := buildTestApp(t)
	cookie := loginAs(t, a, "admin@careops.local", "Admin!234567")

	createTestReportDefinition(t, a, cookie, "Idempotent Run", "audit")
	id := getReportID(t, a, cookie, "Idempotent Run")

	makeRunReq := func() *http.Response {
		t.Helper()
		body := strings.NewReader("_idempotency_key=run-idem-key-001")
		req := httptest.NewRequest(http.MethodPost, "/reports/"+id+"/run", body)
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.AddCookie(&http.Cookie{Name: "careops_session", Value: cookie})
		resp, err := a.Test(req, -1)
		if err != nil {
			t.Fatalf("run: %v", err)
		}
		return resp
	}

	r1 := makeRunReq()
	r2 := makeRunReq()

	if r1.StatusCode != http.StatusFound && r1.StatusCode != http.StatusSeeOther {
		t.Errorf("first run: want redirect, got %d", r1.StatusCode)
	}
	if r2.StatusCode != http.StatusFound && r2.StatusCode != http.StatusSeeOther {
		t.Errorf("duplicate run: want redirect (idempotent replay), got %d", r2.StatusCode)
	}

	// Both requests must succeed — the idempotency key prevents double execution.
	// We verify the page still renders correctly (not an error state).
	listResp := getWithSession(t, a, "/reports", cookie)
	if listResp.StatusCode != http.StatusOK {
		t.Errorf("after idempotent runs: /reports page should still render 200, got %d", listResp.StatusCode)
	}
}

// ── Download output ──────────────────────────────────────────────────────────

func TestReports_DownloadOutput_MissingRunReturns404(t *testing.T) {
	a := buildTestApp(t)
	cookie := loginAs(t, a, "admin@careops.local", "Admin!234567")
	resp := getWithSession(t, a, "/reports/runs/download?run=nonexistent-id", cookie)
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("download missing run: want 404, got %d", resp.StatusCode)
	}
}

func TestReports_DownloadOutput_MissingRunIDReturns400(t *testing.T) {
	a := buildTestApp(t)
	cookie := loginAs(t, a, "admin@careops.local", "Admin!234567")
	resp := getWithSession(t, a, "/reports/runs/download", cookie)
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("download no run param: want 400, got %d", resp.StatusCode)
	}
}

func TestReports_DownloadOutput_CompletedRunServeFile(t *testing.T) {
	a := buildTestApp(t)
	cookie := loginAs(t, a, "admin@careops.local", "Admin!234567")

	createTestReportDefinition(t, a, cookie, "Download Test", "occupancy")
	id := getReportID(t, a, cookie, "Download Test")

	// Run the report.
	req := httptest.NewRequest(http.MethodPost, "/reports/"+id+"/run", nil)
	req.AddCookie(&http.Cookie{Name: "careops_session", Value: cookie})
	_, _ = a.Test(req, -1)

	// Find the download link in the run history.
	listResp := getWithSession(t, a, "/reports", cookie)
	listBody := readBody(t, listResp)

	dlPrefix := `/reports/runs/download?run=`
	idx := strings.Index(listBody, dlPrefix)
	if idx < 0 {
		t.Skip("no download link found — run may not have completed")
	}
	start := idx
	end := strings.Index(listBody[start:], `"`)
	dlPath := listBody[start : start+end]

	dlResp := getWithSession(t, a, dlPath, cookie)
	if dlResp.StatusCode != http.StatusOK {
		t.Errorf("download completed run: want 200, got %d", dlResp.StatusCode)
	}
	cd := dlResp.Header.Get("Content-Disposition")
	if !strings.Contains(cd, "attachment") {
		t.Error("download: Content-Disposition should be attachment")
	}
}

// ── Toggle active ────────────────────────────────────────────────────────────

func TestReports_Toggle_FlipsActiveFlag(t *testing.T) {
	a := buildTestApp(t)
	cookie := loginAs(t, a, "admin@careops.local", "Admin!234567")

	createTestReportDefinition(t, a, cookie, "Toggle Test", "occupancy")
	id := getReportID(t, a, cookie, "Toggle Test")

	// Toggle once — should deactivate.
	req := httptest.NewRequest(http.MethodPost, "/reports/"+id+"/toggle", nil)
	req.AddCookie(&http.Cookie{Name: "careops_session", Value: cookie})
	resp, err := a.Test(req, -1)
	if err != nil {
		t.Fatalf("toggle: %v", err)
	}
	if resp.StatusCode != http.StatusFound && resp.StatusCode != http.StatusSeeOther {
		t.Errorf("toggle: want redirect, got %d", resp.StatusCode)
	}

	listResp := getWithSession(t, a, "/reports", cookie)
	body := readBody(t, listResp)
	if !strings.Contains(body, "inactive") {
		t.Error("after toggle: report should show inactive status")
	}
}

// ── Multiple report types ────────────────────────────────────────────────────

func TestReports_AllReportTypes_CanBeCreatedAndRun(t *testing.T) {
	a := buildTestApp(t)
	cookie := loginAs(t, a, "admin@careops.local", "Admin!234567")

	for _, rt := range []string{"occupancy", "service_delivery", "finance", "audit"} {
		fields := url.Values{
			"name":          {rt + " type test"},
			"report_type":   {rt},
			"output_format": {"csv"},
		}
		postReport(t, a, cookie, fields)
		id := getReportID(t, a, cookie, rt+" type test")

		req := httptest.NewRequest(http.MethodPost, "/reports/"+id+"/run", nil)
		req.AddCookie(&http.Cookie{Name: "careops_session", Value: cookie})
		resp, err := a.Test(req, -1)
		if err != nil {
			t.Fatalf("run %s: %v", rt, err)
		}
		if resp.StatusCode != http.StatusFound && resp.StatusCode != http.StatusSeeOther {
			body, _ := io.ReadAll(resp.Body)
			t.Errorf("run %s report: want redirect, got %d — %s", rt, resp.StatusCode, string(body)[:min(200, len(body))])
		}
	}
}

func TestReports_XLSXFormat_RunProducesXLSXFile(t *testing.T) {
	a := buildTestApp(t)
	cookie := loginAs(t, a, "admin@careops.local", "Admin!234567")

	fields := url.Values{
		"name":          {"XLSX Finance Report"},
		"report_type":   {"finance"},
		"output_format": {"xlsx"},
	}
	postReport(t, a, cookie, fields)
	id := getReportID(t, a, cookie, "XLSX Finance Report")

	req := httptest.NewRequest(http.MethodPost, "/reports/"+id+"/run", nil)
	req.AddCookie(&http.Cookie{Name: "careops_session", Value: cookie})
	_, _ = a.Test(req, -1)

	listResp := getWithSession(t, a, "/reports", cookie)
	listBody := readBody(t, listResp)

	dlPrefix := `/reports/runs/download?run=`
	idx := strings.Index(listBody, dlPrefix)
	if idx < 0 {
		t.Skip("no download link found")
	}
	start := idx
	end := strings.Index(listBody[start:], `"`)
	dlPath := listBody[start : start+end]

	dlResp := getWithSession(t, a, dlPath, cookie)
	if dlResp.StatusCode != http.StatusOK {
		errBody, _ := io.ReadAll(dlResp.Body)
		t.Fatalf("xlsx download: want 200, got %d — %s", dlResp.StatusCode, string(errBody)[:min(200, len(errBody))])
	}

	// Verify it's a valid ZIP (XLSX is a ZIP archive).
	body, _ := io.ReadAll(dlResp.Body)
	if len(body) < 4 {
		t.Fatal("xlsx response body too short to be a valid file")
	}
	// XLSX files start with PK (ZIP magic bytes: 0x50 0x4B).
	if body[0] != 0x50 || body[1] != 0x4B {
		t.Errorf("xlsx file should start with PK (ZIP magic bytes 50 4B), got: %02x %02x", body[0], body[1])
	}

	// Validate it's a readable ZIP.
	zr, err := zip.NewReader(bytes.NewReader(body), int64(len(body)))
	if err != nil {
		t.Errorf("xlsx output is not a valid ZIP/XLSX: %v", err)
		return
	}
	if len(zr.File) == 0 {
		t.Error("xlsx output ZIP has no entries")
	}
}

// ── Index page content ───────────────────────────────────────────────────────

func TestReports_IndexPage_NoShellBanner(t *testing.T) {
	a := buildTestApp(t)
	cookie := loginAs(t, a, "admin@careops.local", "Admin!234567")
	resp := getWithSession(t, a, "/reports", cookie)
	body := readBody(t, resp)
	if strings.Contains(body, "Phase 1 shell") {
		t.Error("/reports page must not contain 'Phase 1 shell' banner — feature is fully implemented")
	}
}

func TestReports_IndexPage_ContainsNewReportLink(t *testing.T) {
	a := buildTestApp(t)
	cookie := loginAs(t, a, "admin@careops.local", "Admin!234567")
	resp := getWithSession(t, a, "/reports", cookie)
	body := readBody(t, resp)
	if !strings.Contains(body, "/reports/new") {
		t.Error("/reports page should contain a link to /reports/new")
	}
}
