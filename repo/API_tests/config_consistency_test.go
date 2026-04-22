package api_tests

// config_consistency_test.go — statically verifies that docker-compose.yml,
// README.md, and config.go agree on configuration defaults and behaviour
// documented in Task 1 (startup-consistency fixes).
//
// These tests read files on disk only — no network, no DB, no app bootstrap.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestCompose_AppEnvIsNotProduction asserts that docker-compose.yml configures
// APP_ENV=development so that `docker-compose up` works out of the box without
// a FIELD_ENCRYPT_KEY.  An active (uncommented) APP_ENV=production line would
// cause startup to hard-fail when the key is absent.
func TestCompose_AppEnvIsNotProduction(t *testing.T) {
	composePath := filepath.Join(repoRoot(), "docker-compose.yml")
	data, err := os.ReadFile(composePath)
	if err != nil {
		t.Fatalf("read docker-compose.yml: %v", err)
	}
	content := string(data)

	// Scan line-by-line so we only flag uncommented lines.
	for i, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#") {
			continue
		}
		if strings.Contains(trimmed, "APP_ENV") && strings.Contains(trimmed, "production") {
			t.Errorf("docker-compose.yml line %d: APP_ENV must not be 'production' — got %q", i+1, trimmed)
		}
	}
}

// TestCompose_AppEnvIsDevelopment asserts that docker-compose.yml has an active
// APP_ENV: "development" setting so the default mode is clearly stated.
func TestCompose_AppEnvIsDevelopment(t *testing.T) {
	composePath := filepath.Join(repoRoot(), "docker-compose.yml")
	data, err := os.ReadFile(composePath)
	if err != nil {
		t.Fatalf("read docker-compose.yml: %v", err)
	}
	found := false
	for _, line := range strings.Split(string(data), "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#") {
			continue
		}
		if strings.Contains(trimmed, "APP_ENV") && strings.Contains(trimmed, "development") {
			found = true
			break
		}
	}
	if !found {
		t.Error("docker-compose.yml: expected an active APP_ENV=development line")
	}
}

// TestREADME_HasConfigTruthTable asserts that the README contains the
// "Configuration Truth Table" section added in Task 1.
func TestREADME_HasConfigTruthTable(t *testing.T) {
	readmePath := filepath.Join(repoRoot(), "README.md")
	data, err := os.ReadFile(readmePath)
	if err != nil {
		t.Fatalf("read README.md: %v", err)
	}
	body := string(data)

	if !strings.Contains(body, "Configuration Truth Table") {
		t.Error("README.md: missing 'Configuration Truth Table' section")
	}
	// The table must cover all four meaningful combinations.
	requiredRows := []string{
		"development",
		"production",
		"hard-fails",
		"unset",
	}
	for _, phrase := range requiredRows {
		if !strings.Contains(strings.ToLower(body), strings.ToLower(phrase)) {
			t.Errorf("README.md truth table: phrase %q not found", phrase)
		}
	}
}

// TestREADME_LogPathDefaultMatchesCode asserts that the README documents the
// LOG_PATH default as "./data/logs/app.log", which must agree with the actual
// default set in internal/config/config.go.
func TestREADME_LogPathDefaultMatchesCode(t *testing.T) {
	const wantDefault = "./data/logs/app.log"
	const wantREADMEPhrase = "./data/logs/app.log"

	// Verify config.go contains the expected default.
	cfgPath := filepath.Join(repoRoot(), "internal", "config", "config.go")
	cfgData, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("read config.go: %v", err)
	}
	if !strings.Contains(string(cfgData), wantDefault) {
		t.Errorf("config.go: LOG_PATH default %q not found — code and docs may be out of sync", wantDefault)
	}

	// Verify README documents the same default.
	readmePath := filepath.Join(repoRoot(), "README.md")
	readmeData, err := os.ReadFile(readmePath)
	if err != nil {
		t.Fatalf("read README.md: %v", err)
	}
	if !strings.Contains(string(readmeData), wantREADMEPhrase) {
		t.Errorf("README.md: LOG_PATH default %q not documented — docs are out of sync with config.go", wantREADMEPhrase)
	}
}

// TestREADME_NoStaleLogPathDefault asserts that the README no longer uses the
// old stale documentation of LOG_PATH that claimed the default was unset.
func TestREADME_NoStaleLogPathDefault(t *testing.T) {
	readmePath := filepath.Join(repoRoot(), "README.md")
	data, err := os.ReadFile(readmePath)
	if err != nil {
		t.Fatalf("read README.md: %v", err)
	}
	body := string(data)

	// The stale row claimed LOG_PATH default was "_(unset)_".
	stalePatterns := []string{
		"LOG_PATH` | `_(unset)_`",
		"LOG_PATH | _(unset)_",
	}
	for _, stale := range stalePatterns {
		if strings.Contains(body, stale) {
			t.Errorf("README.md: stale LOG_PATH default %q must be replaced with the real default", stale)
		}
	}
}
