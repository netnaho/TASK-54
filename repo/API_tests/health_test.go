package api_tests

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"careops/clinic/internal/app"
	"careops/clinic/internal/config"
	"careops/clinic/internal/platform/logger"
)

func TestHealthz(t *testing.T) {
	a := buildTestApp(t)
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	resp, err := a.Test(req, -1)
	if err != nil {
		t.Fatalf("Test /healthz: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	var body map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body["status"] != "ok" {
		t.Errorf(`expected "ok", got %q`, body["status"])
	}
}

func TestReadyz(t *testing.T) {
	a := buildTestApp(t)
	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	resp, err := a.Test(req, -1)
	if err != nil {
		t.Fatalf("Test /readyz: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func buildTestApp(t *testing.T) interface{ Test(*http.Request, ...int) (*http.Response, error) } {
	t.Helper()

	cfg := testConfig(t)
	log := logger.New("text", "error")

	db, err := app.BootstrapFast(cfg, log)
	if err != nil {
		t.Fatalf("Bootstrap: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	return app.New(cfg, db, log)
}

func testConfig(t *testing.T) *config.Config {
	t.Helper()

	root := repoRoot()
	dbPath := t.TempDir() + "/test.db"

	cfg := &config.Config{
		Port:                     "0",
		DBPath:                   dbPath,
		MigrationsPath:           root + "/migrations",
		SeedsPath:                root + "/seeds",
		TemplatesPath:            root + "/web/templates",
		StaticPath:               root + "/web/static",
		MediaPath:                t.TempDir(),
		ExportsPath:              t.TempDir(),
		DiagnosticsPath:          t.TempDir(),
		ConfigSnapPath:           t.TempDir(),
		SessionTTLHours:          24,
		InactivityTimeoutMinutes: 15,
		CookieName:               "careops_session",
		LogLevel:                 "error",
		LogFormat:                "text",
		Env:                      "development",
		FacilityTimezone:         "UTC",
	}
	return cfg
}
