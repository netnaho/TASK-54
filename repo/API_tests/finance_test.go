package api_tests

import (
	"bytes"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

// ── Authorization boundaries ─────────────────────────────────────────────────

func TestFinance_NurseGets403(t *testing.T) {
	a := buildTestApp(t)
	cookie := loginAs(t, a, "nurse@careops.local", "Nurse!234567")
	resp := getWithSession(t, a, "/finance", cookie)
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("nurse on /finance: want 403, got %d", resp.StatusCode)
	}
}

func TestFinance_TherapistGets403(t *testing.T) {
	a := buildTestApp(t)
	cookie := loginAs(t, a, "therapist@careops.local", "Therapist!2345")
	resp := getWithSession(t, a, "/finance", cookie)
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("therapist on /finance: want 403, got %d", resp.StatusCode)
	}
}

func TestFinance_AuditorGets403(t *testing.T) {
	a := buildTestApp(t)
	cookie := loginAs(t, a, "auditor@careops.local", "Auditor!2345")
	resp := getWithSession(t, a, "/finance", cookie)
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("auditor on /finance: want 403, got %d", resp.StatusCode)
	}
}

func TestFinance_FinanceClerkGets200(t *testing.T) {
	a := buildTestApp(t)
	cookie := loginAs(t, a, "finance@careops.local", "Finance!2345")
	resp := getWithSession(t, a, "/finance", cookie)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("finance_clerk on /finance: want 200, got %d", resp.StatusCode)
	}
}

func TestFinance_AdminGets200(t *testing.T) {
	a := buildTestApp(t)
	cookie := loginAs(t, a, "admin@careops.local", "Admin!234567")
	resp := getWithSession(t, a, "/finance", cookie)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("admin on /finance: want 200, got %d", resp.StatusCode)
	}
}

// ── Payment form ──────────────────────────────────────────────────────────────

func TestFinance_PaymentFormGet200(t *testing.T) {
	a := buildTestApp(t)
	cookie := loginAs(t, a, "finance@careops.local", "Finance!2345")
	resp := getWithSession(t, a, "/finance/payments/new", cookie)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("GET /finance/payments/new: want 200, got %d", resp.StatusCode)
	}
}

// ── Payment creation ──────────────────────────────────────────────────────────

func TestFinance_RecordCashPayment(t *testing.T) {
	a := buildTestApp(t)
	cookie := loginAs(t, a, "finance@careops.local", "Finance!2345")

	// Look up the first resident's ID from the seeded data by checking the form page.
	formResp := getWithSession(t, a, "/finance/payments/new", cookie)
	body, _ := io.ReadAll(formResp.Body)
	// Extract a resident ID from the form options.
	// Seeds create residents with IDs starting with 'res-'
	bodyStr := string(body)
	// Find the first occurrence of value="res- in the select options
	idx := strings.Index(bodyStr, `value="res-`)
	if idx < 0 {
		t.Skip("no resident found in payment form — seeds may not have loaded")
	}
	start := idx + len(`value="`)
	end := strings.Index(bodyStr[start:], `"`)
	residentID := bodyStr[start : start+end]

	form := url.Values{}
	form.Set("resident_id", residentID)
	form.Set("payment_method", "cash")
	form.Set("amount", "50.00")
	form.Set("description", "Test payment — cash")

	req := httptest.NewRequest(http.MethodPost, "/finance/payments", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(&http.Cookie{Name: "careops_session", Value: cookie})

	resp, err := a.Test(req, -1)
	if err != nil {
		t.Fatalf("POST /finance/payments: %v", err)
	}
	// Expect redirect to the new payment detail page
	if resp.StatusCode != http.StatusFound && resp.StatusCode != http.StatusSeeOther {
		body, _ := io.ReadAll(resp.Body)
		t.Errorf("POST /finance/payments: want redirect, got %d — %s", resp.StatusCode, string(body)[:min(200, len(body))])
	}
}

func TestFinance_RecordPayment_MissingResident_Returns400(t *testing.T) {
	a := buildTestApp(t)
	cookie := loginAs(t, a, "finance@careops.local", "Finance!2345")

	form := url.Values{}
	form.Set("resident_id", "")
	form.Set("payment_method", "cash")
	form.Set("amount", "50.00")
	form.Set("description", "test")

	req := httptest.NewRequest(http.MethodPost, "/finance/payments", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(&http.Cookie{Name: "careops_session", Value: cookie})

	resp, _ := a.Test(req, -1)
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("missing resident: want 400, got %d", resp.StatusCode)
	}
}

func TestFinance_RecordPayment_NonFinanceGets403(t *testing.T) {
	a := buildTestApp(t)
	cookie := loginAs(t, a, "nurse@careops.local", "Nurse!234567")

	form := url.Values{}
	form.Set("resident_id", "res-001")
	form.Set("payment_method", "cash")
	form.Set("amount", "50.00")
	form.Set("description", "test")

	req := httptest.NewRequest(http.MethodPost, "/finance/payments", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(&http.Cookie{Name: "careops_session", Value: cookie})

	resp, _ := a.Test(req, -1)
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("nurse POST /finance/payments: want 403, got %d", resp.StatusCode)
	}
}

// ── Shift management ─────────────────────────────────────────────────────────

func TestFinance_ShiftList200(t *testing.T) {
	a := buildTestApp(t)
	cookie := loginAs(t, a, "finance@careops.local", "Finance!2345")
	resp := getWithSession(t, a, "/finance/shifts", cookie)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("GET /finance/shifts: want 200, got %d", resp.StatusCode)
	}
}

func TestFinance_OpenShift(t *testing.T) {
	a := buildTestApp(t)
	cookie := loginAs(t, a, "finance@careops.local", "Finance!2345")

	form := url.Values{}
	form.Set("shift_name", "morning")

	req := httptest.NewRequest(http.MethodPost, "/finance/shifts", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(&http.Cookie{Name: "careops_session", Value: cookie})

	resp, err := a.Test(req, -1)
	if err != nil {
		t.Fatalf("POST /finance/shifts: %v", err)
	}
	if resp.StatusCode != http.StatusFound && resp.StatusCode != http.StatusSeeOther {
		t.Errorf("open shift: want redirect, got %d", resp.StatusCode)
	}
}

// ── Card batch import ─────────────────────────────────────────────────────────

func TestFinance_BatchImportFormGet200(t *testing.T) {
	a := buildTestApp(t)
	cookie := loginAs(t, a, "finance@careops.local", "Finance!2345")
	resp := getWithSession(t, a, "/finance/batches/new", cookie)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("GET /finance/batches/new: want 200, got %d", resp.StatusCode)
	}
}

func TestFinance_BatchImport_NoFileFails(t *testing.T) {
	a := buildTestApp(t)
	cookie := loginAs(t, a, "finance@careops.local", "Finance!2345")

	req := httptest.NewRequest(http.MethodPost, "/finance/batches", strings.NewReader(""))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(&http.Cookie{Name: "careops_session", Value: cookie})

	resp, _ := a.Test(req, -1)
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("no file: want 400, got %d", resp.StatusCode)
	}
}

func TestFinance_BatchImport_CSVSuccess(t *testing.T) {
	a := buildTestApp(t)
	cookie := loginAs(t, a, "finance@careops.local", "Finance!2345")

	// Get a real resident MRN from the seeded data.
	admResp := getWithSession(t, a, "/admissions", cookie)
	body, _ := io.ReadAll(admResp.Body)
	bodyStr := string(body)
	idx := strings.Index(bodyStr, "MRN-")
	if idx < 0 {
		t.Skip("no MRN found in admissions page — seeds may not have loaded")
	}
	// Extract the MRN (up to the next quote or space)
	mrnStart := idx
	mrnEnd := mrnStart
	for mrnEnd < len(bodyStr) && bodyStr[mrnEnd] != '"' && bodyStr[mrnEnd] != '<' && bodyStr[mrnEnd] != ' ' {
		mrnEnd++
	}
	mrn := bodyStr[mrnStart:mrnEnd]

	csvData := "resident_mrn,amount_dollars,description,reference_number\n" +
		mrn + ",25.00,Batch test payment,TERM-001\n"

	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	fw, _ := w.CreateFormFile("batch_file", "test_batch.csv")
	fw.Write([]byte(csvData))
	w.Close()

	req := httptest.NewRequest(http.MethodPost, "/finance/batches", &buf)
	req.Header.Set("Content-Type", w.FormDataContentType())
	req.AddCookie(&http.Cookie{Name: "careops_session", Value: cookie})

	resp, err := a.Test(req, -1)
	if err != nil {
		t.Fatalf("POST /finance/batches: %v", err)
	}
	if resp.StatusCode != http.StatusFound && resp.StatusCode != http.StatusSeeOther {
		body, _ := io.ReadAll(resp.Body)
		t.Errorf("batch import: want redirect, got %d — %s", resp.StatusCode, string(body)[:min(300, len(body))])
	}
}

// ── Export generation ─────────────────────────────────────────────────────────

func TestFinance_ExportFormGet200(t *testing.T) {
	a := buildTestApp(t)
	cookie := loginAs(t, a, "finance@careops.local", "Finance!2345")
	resp := getWithSession(t, a, "/finance/exports/new", cookie)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("GET /finance/exports/new: want 200, got %d", resp.StatusCode)
	}
}

func TestFinance_ExportCreate_CSVRedirectsToDownload(t *testing.T) {
	a := buildTestApp(t)
	cookie := loginAs(t, a, "finance@careops.local", "Finance!2345")

	form := url.Values{}
	form.Set("report_type", "finance")
	form.Set("format", "csv")

	req := httptest.NewRequest(http.MethodPost, "/finance/exports", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(&http.Cookie{Name: "careops_session", Value: cookie})

	resp, err := a.Test(req, -1)
	if err != nil {
		t.Fatalf("POST /finance/exports: %v", err)
	}
	if resp.StatusCode != http.StatusFound && resp.StatusCode != http.StatusSeeOther {
		t.Errorf("export create CSV: want redirect, got %d", resp.StatusCode)
	}
	loc := resp.Header.Get("Location")
	if !strings.HasPrefix(loc, "/finance/exports/download") {
		t.Errorf("export create: redirect should be to /finance/exports/download, got %q", loc)
	}
}

func TestFinance_ExportCreate_XLSXRedirectsToDownload(t *testing.T) {
	a := buildTestApp(t)
	cookie := loginAs(t, a, "finance@careops.local", "Finance!2345")

	form := url.Values{}
	form.Set("report_type", "finance")
	form.Set("format", "xlsx")

	req := httptest.NewRequest(http.MethodPost, "/finance/exports", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(&http.Cookie{Name: "careops_session", Value: cookie})

	resp, err := a.Test(req, -1)
	if err != nil {
		t.Fatalf("POST /finance/exports xlsx: %v", err)
	}
	if resp.StatusCode != http.StatusFound && resp.StatusCode != http.StatusSeeOther {
		t.Errorf("export create XLSX: want redirect, got %d", resp.StatusCode)
	}
}

func TestFinance_ExportDownload_PathTraversalBlocked(t *testing.T) {
	a := buildTestApp(t)
	cookie := loginAs(t, a, "finance@careops.local", "Finance!2345")

	// filepath.Base sanitizes traversal — result is 404 (file not found) or 400.
	resp := getWithSession(t, a, "/finance/exports/download?file=../../etc/passwd", cookie)
	if resp.StatusCode == http.StatusOK || resp.StatusCode >= 500 {
		t.Errorf("path traversal should not return %d", resp.StatusCode)
	}
}

func TestFinance_ExportDownload_MissingFileReturns404(t *testing.T) {
	a := buildTestApp(t)
	cookie := loginAs(t, a, "finance@careops.local", "Finance!2345")
	resp := getWithSession(t, a, "/finance/exports/download?file=nonexistent.csv", cookie)
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("missing export file: want 404, got %d", resp.StatusCode)
	}
}

// ── Non-finance user cannot reach payment sub-pages ───────────────────────────

func TestFinance_SubPages_RequireFinanceRole(t *testing.T) {
	a := buildTestApp(t)
	cookie := loginAs(t, a, "auditor@careops.local", "Auditor!2345")

	paths := []string{
		"/finance/payments/new",
		"/finance/shifts",
		"/finance/batches/new",
		"/finance/exports/new",
	}
	for _, p := range paths {
		resp := getWithSession(t, a, p, cookie)
		if resp.StatusCode != http.StatusForbidden {
			t.Errorf("auditor on %s: want 403, got %d", p, resp.StatusCode)
		}
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
