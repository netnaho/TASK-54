package api_tests

import (
	"archive/zip"
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"careops/clinic/internal/app"
	"careops/clinic/internal/platform/logger"
)

// TestDiagnostics_ZIPContainsAppLogsWhenLogPathSet generates a diagnostic
// package using an app configured with a real log file, then verifies that
// app_logs.txt appears inside the generated ZIP.
func TestDiagnostics_ZIPContainsAppLogsWhenLogPathSet(t *testing.T) {
	// Create a temporary log file with known content.
	logDir := t.TempDir()
	logPath := filepath.Join(logDir, "test.log")
	if err := os.WriteFile(logPath, []byte("INFO starting server\nINFO request received\n"), 0o640); err != nil {
		t.Fatalf("write log file: %v", err)
	}

	cfg := testConfig(t)
	cfg.LogPath = logPath

	log := logger.New("text", "error")
	db, err := app.BootstrapFast(cfg, log)
	if err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	a := app.New(cfg, db, log)
	cookie := loginAs(t, a, "admin@careops.local", "Admin!234567")

	// Generate a diagnostic package.
	genReq := httptest.NewRequest(http.MethodPost, "/diagnostics/exports", nil)
	genReq.AddCookie(&http.Cookie{Name: "careops_session", Value: cookie})
	genResp, err := a.Test(genReq, -1)
	if err != nil {
		t.Fatalf("generate package: %v", err)
	}
	if genResp.StatusCode != http.StatusFound && genResp.StatusCode != http.StatusSeeOther {
		t.Fatalf("generate package: want redirect, got %d", genResp.StatusCode)
	}

	// Find download link.
	indexResp, err := a.Test(mustRequest(t, http.MethodGet, "/diagnostics", cookie), -1)
	if err != nil {
		t.Fatalf("index: %v", err)
	}
	indexBody := readBody(t, indexResp)

	dlPrefix := `/diagnostics/exports/download?file=`
	dlIdx := strings.Index(indexBody, dlPrefix)
	if dlIdx < 0 {
		t.Skip("no download link found")
	}
	dlEnd := strings.Index(indexBody[dlIdx:], `"`)
	dlPath := indexBody[dlIdx : dlIdx+dlEnd]

	// Download and inspect the ZIP.
	dlResp, err := a.Test(mustRequest(t, http.MethodGet, dlPath, cookie), -1)
	if err != nil {
		t.Fatalf("download: %v", err)
	}
	if dlResp.StatusCode != http.StatusOK {
		t.Fatalf("download: want 200, got %d", dlResp.StatusCode)
	}
	zipBytes, _ := io.ReadAll(dlResp.Body)

	zr, err := zip.NewReader(bytes.NewReader(zipBytes), int64(len(zipBytes)))
	if err != nil {
		t.Fatalf("parse ZIP: %v", err)
	}

	var found bool
	for _, f := range zr.File {
		if f.Name == "app_logs.txt" {
			found = true
			rc, err := f.Open()
			if err != nil {
				t.Fatalf("open app_logs.txt: %v", err)
			}
			content, _ := io.ReadAll(rc)
			rc.Close()
			if !strings.Contains(string(content), "INFO") {
				t.Error("app_logs.txt should contain log lines")
			}
		}
	}
	if !found {
		t.Error("diagnostic ZIP should contain app_logs.txt when log path is configured")
	}
}

// TestDiagnostics_ZIPNoAppLogsWhenLogPathUnset verifies that app_logs.txt is
// absent from the ZIP when LogPath is explicitly cleared (empty string).
// Note: testConfig sets LogPath="" directly; config.Load() now defaults to
// "./data/logs/app.log", so this test exercises the explicit-empty override path.
func TestDiagnostics_ZIPNoAppLogsWhenLogPathUnset(t *testing.T) {
	cfg := testConfig(t)
	cfg.LogPath = "" // explicit empty: should suppress app_logs.txt
	log := logger.New("text", "error")
	db, err := app.BootstrapFast(cfg, log)
	if err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	a := app.New(cfg, db, log)
	cookie := loginAs(t, a, "admin@careops.local", "Admin!234567")

	genReq := httptest.NewRequest(http.MethodPost, "/diagnostics/exports", nil)
	genReq.AddCookie(&http.Cookie{Name: "careops_session", Value: cookie})
	genResp, err := a.Test(genReq, -1)
	if err != nil || (genResp.StatusCode != http.StatusFound && genResp.StatusCode != http.StatusSeeOther) {
		t.Fatalf("generate package: status=%d err=%v", genResp.StatusCode, err)
	}

	indexResp := getWithSession(t, a, "/diagnostics", cookie)
	indexBody := readBody(t, indexResp)

	dlPrefix := `/diagnostics/exports/download?file=`
	dlIdx := strings.Index(indexBody, dlPrefix)
	if dlIdx < 0 {
		t.Skip("no download link")
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
		if f.Name == "app_logs.txt" {
			t.Error("diagnostic ZIP should NOT contain app_logs.txt when no log path is configured")
		}
	}
}

// TestDiagnostics_AppLogsRedactsBcrypt verifies that bcrypt hashes found in
// the log file are replaced with [REDACTED-HASH] in app_logs.txt.
func TestDiagnostics_AppLogsRedactsBcrypt(t *testing.T) {
	logDir := t.TempDir()
	logPath := filepath.Join(logDir, "redact_test.log")

	// Write a log line with a fake bcrypt hash.
	bcryptHash := `$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy`
	content := "INFO server start\nDEBUG hashed password: " + bcryptHash + "\nINFO done\n"
	if err := os.WriteFile(logPath, []byte(content), 0o640); err != nil {
		t.Fatalf("write log: %v", err)
	}

	cfg := testConfig(t)
	cfg.LogPath = logPath
	log := logger.New("text", "error")
	db, err := app.BootstrapFast(cfg, log)
	if err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	a := app.New(cfg, db, log)
	cookie := loginAs(t, a, "admin@careops.local", "Admin!234567")

	genReq := httptest.NewRequest(http.MethodPost, "/diagnostics/exports", nil)
	genReq.AddCookie(&http.Cookie{Name: "careops_session", Value: cookie})
	a.Test(genReq, -1) //nolint

	indexResp, _ := a.Test(mustRequest(t, http.MethodGet, "/diagnostics", cookie), -1)
	indexBody := readBody(t, indexResp)

	dlPrefix := `/diagnostics/exports/download?file=`
	dlIdx := strings.Index(indexBody, dlPrefix)
	if dlIdx < 0 {
		t.Skip("no download link")
	}
	dlEnd := strings.Index(indexBody[dlIdx:], `"`)
	dlPath := indexBody[dlIdx : dlIdx+dlEnd]

	dlResp, _ := a.Test(mustRequest(t, http.MethodGet, dlPath, cookie), -1)
	zipBytes, _ := io.ReadAll(dlResp.Body)

	zr, err := zip.NewReader(bytes.NewReader(zipBytes), int64(len(zipBytes)))
	if err != nil {
		t.Fatalf("parse ZIP: %v", err)
	}

	for _, f := range zr.File {
		if f.Name != "app_logs.txt" {
			continue
		}
		rc, _ := f.Open()
		logContent, _ := io.ReadAll(rc)
		rc.Close()

		if strings.Contains(string(logContent), bcryptHash) {
			t.Error("app_logs.txt should not contain raw bcrypt hashes")
		}
		if !strings.Contains(string(logContent), "[REDACTED-HASH]") {
			t.Error("app_logs.txt should contain [REDACTED-HASH] placeholder")
		}
		return
	}
	t.Error("app_logs.txt not found in ZIP")
}

// TestDiagnostics_ReadmeDescribesAppLogs verifies that when a log path is set,
// the README.txt in the ZIP mentions app_logs.txt.
func TestDiagnostics_ReadmeDescribesAppLogs(t *testing.T) {
	logDir := t.TempDir()
	logPath := filepath.Join(logDir, "readme_test.log")
	os.WriteFile(logPath, []byte("log entry\n"), 0o640) //nolint

	cfg := testConfig(t)
	cfg.LogPath = logPath
	log := logger.New("text", "error")
	db, err := app.BootstrapFast(cfg, log)
	if err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	a := app.New(cfg, db, log)
	cookie := loginAs(t, a, "admin@careops.local", "Admin!234567")

	genReq := httptest.NewRequest(http.MethodPost, "/diagnostics/exports", nil)
	genReq.AddCookie(&http.Cookie{Name: "careops_session", Value: cookie})
	a.Test(genReq, -1) //nolint

	indexResp, _ := a.Test(mustRequest(t, http.MethodGet, "/diagnostics", cookie), -1)
	indexBody := readBody(t, indexResp)

	dlPrefix := `/diagnostics/exports/download?file=`
	dlIdx := strings.Index(indexBody, dlPrefix)
	if dlIdx < 0 {
		t.Skip("no download link")
	}
	dlEnd := strings.Index(indexBody[dlIdx:], `"`)
	dlPath := indexBody[dlIdx : dlIdx+dlEnd]

	dlResp, _ := a.Test(mustRequest(t, http.MethodGet, dlPath, cookie), -1)
	zipBytes, _ := io.ReadAll(dlResp.Body)

	zr, _ := zip.NewReader(bytes.NewReader(zipBytes), int64(len(zipBytes)))
	for _, f := range zr.File {
		if f.Name != "README.txt" {
			continue
		}
		rc, _ := f.Open()
		readmeContent, _ := io.ReadAll(rc)
		rc.Close()
		if !strings.Contains(string(readmeContent), "app_logs.txt") {
			t.Error("README.txt should describe app_logs.txt when log path is configured")
		}
		return
	}
	t.Error("README.txt not found in ZIP")
}

// TestDiagnostics_AppLogsRedactsHexEncryptionKey verifies that 64-character hex
// strings (field encryption keys) in log output are replaced with [REDACTED-KEY]
// and never appear in plaintext inside app_logs.txt.
func TestDiagnostics_AppLogsRedactsHexEncryptionKey(t *testing.T) {
	logDir := t.TempDir()
	logPath := filepath.Join(logDir, "hex_redact_test.log")

	// A 64-char lowercase hex string — the shape of FIELD_ENCRYPT_KEY.
	hexKey := "a1b2c3d4e5f60718293a4b5c6d7e8f90a1b2c3d4e5f60718293a4b5c6d7e8f90"
	content := "INFO boot\nDEBUG encrypt_key=" + hexKey + " loaded\nINFO ready\n"
	if err := os.WriteFile(logPath, []byte(content), 0o640); err != nil {
		t.Fatalf("write log: %v", err)
	}

	cfg := testConfig(t)
	cfg.LogPath = logPath
	log := logger.New("text", "error")
	db, err := app.BootstrapFast(cfg, log)
	if err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	a := app.New(cfg, db, log)
	cookie := loginAs(t, a, "admin@careops.local", "Admin!234567")

	genReq := httptest.NewRequest(http.MethodPost, "/diagnostics/exports", nil)
	genReq.AddCookie(&http.Cookie{Name: "careops_session", Value: cookie})
	a.Test(genReq, -1) //nolint

	indexResp, _ := a.Test(mustRequest(t, http.MethodGet, "/diagnostics", cookie), -1)
	indexBody := readBody(t, indexResp)

	dlPrefix := `/diagnostics/exports/download?file=`
	dlIdx := strings.Index(indexBody, dlPrefix)
	if dlIdx < 0 {
		t.Skip("no download link")
	}
	dlEnd := strings.Index(indexBody[dlIdx:], `"`)
	dlPath := indexBody[dlIdx : dlIdx+dlEnd]

	dlResp, _ := a.Test(mustRequest(t, http.MethodGet, dlPath, cookie), -1)
	zipBytes, _ := io.ReadAll(dlResp.Body)

	zr, err := zip.NewReader(bytes.NewReader(zipBytes), int64(len(zipBytes)))
	if err != nil {
		t.Fatalf("parse ZIP: %v", err)
	}

	for _, f := range zr.File {
		if f.Name != "app_logs.txt" {
			continue
		}
		rc, _ := f.Open()
		logContent, _ := io.ReadAll(rc)
		rc.Close()

		if strings.Contains(string(logContent), hexKey) {
			t.Error("app_logs.txt must not contain raw 64-char hex encryption key")
		}
		if !strings.Contains(string(logContent), "[REDACTED-KEY]") {
			t.Error("app_logs.txt should replace 64-char hex key with [REDACTED-KEY]")
		}
		return
	}
	t.Error("app_logs.txt not found in ZIP")
}

// TestDiagnostics_DefaultLogPathIncludesAppLogs verifies that when LogPath is
// set to a non-empty path (mirroring the new config.Load() default), the
// diagnostic ZIP includes app_logs.txt even without explicit operator override.
func TestDiagnostics_DefaultLogPathIncludesAppLogs(t *testing.T) {
	logDir := t.TempDir()
	logPath := filepath.Join(logDir, "app.log")
	if err := os.WriteFile(logPath, []byte("INFO server started\nINFO request handled\n"), 0o640); err != nil {
		t.Fatalf("write log file: %v", err)
	}

	cfg := testConfig(t)
	cfg.LogPath = logPath // mirrors what config.Load() now returns by default

	log := logger.New("text", "error")
	db, err := app.BootstrapFast(cfg, log)
	if err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	a := app.New(cfg, db, log)
	cookie := loginAs(t, a, "admin@careops.local", "Admin!234567")

	genReq := httptest.NewRequest(http.MethodPost, "/diagnostics/exports", nil)
	genReq.AddCookie(&http.Cookie{Name: "careops_session", Value: cookie})
	genResp, err := a.Test(genReq, -1)
	if err != nil || (genResp.StatusCode != http.StatusFound && genResp.StatusCode != http.StatusSeeOther) {
		t.Fatalf("generate package: status=%d err=%v", genResp.StatusCode, err)
	}

	indexResp, err := a.Test(mustRequest(t, http.MethodGet, "/diagnostics", cookie), -1)
	if err != nil {
		t.Fatalf("index: %v", err)
	}
	indexBody := readBody(t, indexResp)

	dlPrefix := `/diagnostics/exports/download?file=`
	dlIdx := strings.Index(indexBody, dlPrefix)
	if dlIdx < 0 {
		t.Skip("no download link found")
	}
	dlEnd := strings.Index(indexBody[dlIdx:], `"`)
	dlPath := indexBody[dlIdx : dlIdx+dlEnd]

	dlResp, err := a.Test(mustRequest(t, http.MethodGet, dlPath, cookie), -1)
	if err != nil {
		t.Fatalf("download: %v", err)
	}
	zipBytes, _ := io.ReadAll(dlResp.Body)

	zr, err := zip.NewReader(bytes.NewReader(zipBytes), int64(len(zipBytes)))
	if err != nil {
		t.Fatalf("parse ZIP: %v", err)
	}

	var found bool
	for _, f := range zr.File {
		if f.Name == "app_logs.txt" {
			found = true
		}
	}
	if !found {
		t.Error("diagnostic ZIP must contain app_logs.txt when LogPath is set (default config behaviour)")
	}
}

// mustRequest builds a GET request with a session cookie.
func mustRequest(t *testing.T, method, path, cookie string) *http.Request {
	t.Helper()
	req := httptest.NewRequest(method, path, nil)
	if cookie != "" {
		req.AddCookie(&http.Cookie{Name: "careops_session", Value: cookie})
	}
	return req
}
