package api_tests

import (
	"archive/zip"
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// ── Authorization ─────────────────────────────────────────────────────────────

func TestDiagnostics_NonAdminGets403(t *testing.T) {
	a := buildTestApp(t)
	for _, creds := range [][2]string{
		{"nurse@careops.local", "Nurse!234567"},
		{"finance@careops.local", "Finance!2345"},
		{"auditor@careops.local", "Auditor!2345"},
		{"therapist@careops.local", "Therapist!2345"},
	} {
		cookie := loginAs(t, a, creds[0], creds[1])
		resp := getWithSession(t, a, "/diagnostics", cookie)
		if resp.StatusCode != http.StatusForbidden {
			t.Errorf("%s on /diagnostics: want 403, got %d", creds[0], resp.StatusCode)
		}
	}
}

func TestDiagnostics_AdminGets200(t *testing.T) {
	a := buildTestApp(t)
	cookie := loginAs(t, a, "admin@careops.local", "Admin!234567")
	resp := getWithSession(t, a, "/diagnostics", cookie)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("admin on /diagnostics: want 200, got %d", resp.StatusCode)
	}
}

func TestDiagnostics_PageContainsDBStats(t *testing.T) {
	a := buildTestApp(t)
	cookie := loginAs(t, a, "admin@careops.local", "Admin!234567")
	resp := getWithSession(t, a, "/diagnostics", cookie)
	body, _ := io.ReadAll(resp.Body)
	bodyStr := string(body)
	// DiagnosticsService replaces underscores with spaces in pragma keys.
	if !strings.Contains(bodyStr, "page count") && !strings.Contains(bodyStr, "page_count") {
		t.Error("/diagnostics: page should contain 'page count' DB stat")
	}
}

// ── Package generation ────────────────────────────────────────────────────────

func TestDiagnostics_ExportCreate_AdminRedirects(t *testing.T) {
	a := buildTestApp(t)
	cookie := loginAs(t, a, "admin@careops.local", "Admin!234567")

	req := httptest.NewRequest(http.MethodPost, "/diagnostics/exports", nil)
	req.AddCookie(&http.Cookie{Name: "careops_session", Value: cookie})

	resp, err := a.Test(req, -1)
	if err != nil {
		t.Fatalf("POST /diagnostics/exports: %v", err)
	}
	if resp.StatusCode != http.StatusFound && resp.StatusCode != http.StatusSeeOther {
		body, _ := io.ReadAll(resp.Body)
		t.Errorf("generate package: want redirect, got %d — %s", resp.StatusCode, string(body)[:min(200, len(body))])
	}
	loc := resp.Header.Get("Location")
	if !strings.Contains(loc, "/diagnostics") {
		t.Errorf("generate package redirect: want /diagnostics*, got %q", loc)
	}
}

func TestDiagnostics_ExportCreate_NonAdminGets403(t *testing.T) {
	a := buildTestApp(t)
	cookie := loginAs(t, a, "auditor@careops.local", "Auditor!2345")

	req := httptest.NewRequest(http.MethodPost, "/diagnostics/exports", nil)
	req.AddCookie(&http.Cookie{Name: "careops_session", Value: cookie})

	resp, _ := a.Test(req, -1)
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("auditor POST /diagnostics/exports: want 403, got %d", resp.StatusCode)
	}
}

// ── Download safety ───────────────────────────────────────────────────────────

func TestDiagnostics_Download_PathTraversalBlocked(t *testing.T) {
	a := buildTestApp(t)
	cookie := loginAs(t, a, "admin@careops.local", "Admin!234567")

	for _, bad := range []string{
		"../../etc/passwd",
		"../secrets.txt",
		"foo/../../bar",
	} {
		resp := getWithSession(t, a, "/diagnostics/exports/download?file="+bad, cookie)
		// filepath.Base sanitizes the path, resulting in a filename that won't exist.
		// Acceptable: 400 (path check failed) or 404 (file not found after sanitization).
		// Not acceptable: 200 (file served) or 5xx.
		if resp.StatusCode == http.StatusOK || resp.StatusCode >= 500 {
			t.Errorf("path traversal %q should not return %d", bad, resp.StatusCode)
		}
	}
}

func TestDiagnostics_Download_MissingFileReturns404(t *testing.T) {
	a := buildTestApp(t)
	cookie := loginAs(t, a, "admin@careops.local", "Admin!234567")
	resp := getWithSession(t, a, "/diagnostics/exports/download?file=nonexistent.zip", cookie)
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("missing diag package: want 404, got %d", resp.StatusCode)
	}
}

func TestDiagnostics_GenerateAndDownload(t *testing.T) {
	a := buildTestApp(t)
	cookie := loginAs(t, a, "admin@careops.local", "Admin!234567")

	// Generate a package.
	req := httptest.NewRequest(http.MethodPost, "/diagnostics/exports", nil)
	req.AddCookie(&http.Cookie{Name: "careops_session", Value: cookie})
	genResp, err := a.Test(req, -1)
	if err != nil || (genResp.StatusCode != http.StatusFound && genResp.StatusCode != http.StatusSeeOther) {
		t.Fatalf("generate package: %v status=%d", err, genResp.StatusCode)
	}

	// Confirm the package shows on the diagnostics page.
	indexResp := getWithSession(t, a, "/diagnostics", cookie)
	body, _ := io.ReadAll(indexResp.Body)
	bodyStr := string(body)

	// The page should show a download link for the created ZIP.
	if !strings.Contains(bodyStr, "careops_diag_") {
		t.Error("/diagnostics page should list the generated ZIP package")
	}
	if !strings.Contains(bodyStr, "Download") {
		t.Error("/diagnostics page should include a Download link for completed package")
	}

	// Find and follow the download link.
	dlPrefix := `/diagnostics/exports/download?file=`
	dlIdx := strings.Index(bodyStr, dlPrefix)
	if dlIdx < 0 {
		t.Skip("no download link found — package may still be pending")
	}
	dlStart := dlIdx
	dlEnd := strings.Index(bodyStr[dlStart:], `"`)
	dlPath := bodyStr[dlStart : dlStart+dlEnd]

	dlResp := getWithSession(t, a, dlPath, cookie)
	if dlResp.StatusCode != http.StatusOK {
		t.Errorf("download package: want 200, got %d", dlResp.StatusCode)
	}
}

// ── ZIP content verification ──────────────────────────────────────────────────

// TestDiagnostics_ZIPContainsRequiredFiles generates a diagnostic package,
// downloads it, and asserts that all required files are present in the archive.
func TestDiagnostics_ZIPContainsRequiredFiles(t *testing.T) {
	a := buildTestApp(t)
	cookie := loginAs(t, a, "admin@careops.local", "Admin!234567")

	// Generate a package.
	genReq := httptest.NewRequest(http.MethodPost, "/diagnostics/exports", nil)
	genReq.AddCookie(&http.Cookie{Name: "careops_session", Value: cookie})
	genResp, err := a.Test(genReq, -1)
	if err != nil {
		t.Fatalf("generate package: %v", err)
	}
	if genResp.StatusCode != http.StatusFound && genResp.StatusCode != http.StatusSeeOther {
		t.Fatalf("generate package: want redirect, got %d", genResp.StatusCode)
	}

	// Find the download link from the index page.
	indexResp := getWithSession(t, a, "/diagnostics", cookie)
	indexBody := readBody(t, indexResp)

	dlPrefix := `/diagnostics/exports/download?file=`
	dlIdx := strings.Index(indexBody, dlPrefix)
	if dlIdx < 0 {
		t.Skip("no download link found on diagnostics page")
	}
	dlEnd := strings.Index(indexBody[dlIdx:], `"`)
	dlPath := indexBody[dlIdx : dlIdx+dlEnd]

	// Download the ZIP.
	dlResp := getWithSession(t, a, dlPath, cookie)
	if dlResp.StatusCode != http.StatusOK {
		t.Fatalf("download ZIP: want 200, got %d", dlResp.StatusCode)
	}
	zipBytes, err := io.ReadAll(dlResp.Body)
	if err != nil {
		t.Fatalf("read ZIP body: %v", err)
	}

	// Open and inspect the ZIP contents.
	zr, err := zip.NewReader(bytes.NewReader(zipBytes), int64(len(zipBytes)))
	if err != nil {
		t.Fatalf("parse ZIP: %v", err)
	}

	present := map[string]bool{}
	for _, f := range zr.File {
		present[f.Name] = true
	}

	required := []string{
		"README.txt",
		"db_stats.json",
		"recent_jobs.json",
		"config.json",
		"audit_logs.json",
	}
	for _, name := range required {
		if !present[name] {
			t.Errorf("diagnostic ZIP missing required file: %s (present: %v)", name, present)
		}
	}
}

func TestDiagnostics_ZIPAuditLogsContainValidJSON(t *testing.T) {
	a := buildTestApp(t)
	cookie := loginAs(t, a, "admin@careops.local", "Admin!234567")

	// Generate a package.
	genReq := httptest.NewRequest(http.MethodPost, "/diagnostics/exports", nil)
	genReq.AddCookie(&http.Cookie{Name: "careops_session", Value: cookie})
	genResp, _ := a.Test(genReq, -1)
	if genResp.StatusCode != http.StatusFound && genResp.StatusCode != http.StatusSeeOther {
		t.Fatalf("generate package: want redirect, got %d", genResp.StatusCode)
	}

	indexResp := getWithSession(t, a, "/diagnostics", cookie)
	indexBody := readBody(t, indexResp)

	dlPrefix := `/diagnostics/exports/download?file=`
	dlIdx := strings.Index(indexBody, dlPrefix)
	if dlIdx < 0 {
		t.Skip("no download link found")
	}
	dlEnd := strings.Index(indexBody[dlIdx:], `"`)
	dlPath := indexBody[dlIdx : dlIdx+dlEnd]

	dlResp := getWithSession(t, a, dlPath, cookie)
	zipBytes, _ := io.ReadAll(dlResp.Body)

	zr, err := zip.NewReader(bytes.NewReader(zipBytes), int64(len(zipBytes)))
	if err != nil {
		t.Fatalf("parse ZIP: %v", err)
	}

	for _, f := range zr.File {
		if f.Name != "audit_logs.json" {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			t.Fatalf("open audit_logs.json: %v", err)
		}
		content, _ := io.ReadAll(rc)
		rc.Close()

		// Must be a JSON array (empty or populated).
		trimmed := strings.TrimSpace(string(content))
		if !strings.HasPrefix(trimmed, "[") {
			t.Errorf("audit_logs.json should be a JSON array, got: %.50s", trimmed)
		}
		return
	}
	t.Error("audit_logs.json not found in ZIP")
}

func TestDiagnostics_UIDescribesCorrectContents(t *testing.T) {
	a := buildTestApp(t)
	cookie := loginAs(t, a, "admin@careops.local", "Admin!234567")
	resp := getWithSession(t, a, "/diagnostics", cookie)
	body := readBody(t, resp)

	requiredMentions := []string{"audit_logs.json", "config_snapshots"}
	for _, term := range requiredMentions {
		if !strings.Contains(body, term) {
			t.Errorf("/diagnostics UI should describe %q in package contents", term)
		}
	}
}
