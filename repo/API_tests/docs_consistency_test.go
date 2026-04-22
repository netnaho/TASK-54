package api_tests

// Issue 6 — Reports UI scheduler text accuracy
// Issue 7 — README security/traceability docs accuracy
//
// These tests guard against stale documentation by asserting that known-correct
// wording is present and known-wrong (previously stale) wording is absent.

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// Issue 6: Reports new-form UI text
// ---------------------------------------------------------------------------

// TestReportsUI_SchedulerHelpTextIsAccurate asserts that the /reports/new page
// shows accurate scheduler help text and does not contain the old stale wording
// that said automated scheduling was "not yet active".
func TestReportsUI_SchedulerHelpTextIsAccurate(t *testing.T) {
	a := buildTestApp(t)
	cookie := loginAs(t, a, "admin@careops.local", "Admin!234567")

	resp := getWithSession(t, a, "/reports/new", cookie)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /reports/new: want 200, got %d", resp.StatusCode)
	}
	body := readBody(t, resp)

	staleTexts := []string{
		"not yet active",
		"automated scheduling is not yet active",
		"Currently used as documentation",
		"applied in a future release",
	}
	for _, stale := range staleTexts {
		if strings.Contains(body, stale) {
			t.Errorf("reports/new: stale text %q must be removed", stale)
		}
	}

	accurateTexts := []string{
		"runs automatically",
		"Supported keys",
		"date_from",
		"resident_id",
		"Applied as WHERE filters",
	}
	for _, want := range accurateTexts {
		if !strings.Contains(body, want) {
			t.Errorf("reports/new: accurate help text %q not found", want)
		}
	}
}

// TestReportsUI_ParametersHelpTextListsAllSupportedKeys checks that every
// documented filter key appears in the parameters help text on the form.
func TestReportsUI_ParametersHelpTextListsAllSupportedKeys(t *testing.T) {
	a := buildTestApp(t)
	cookie := loginAs(t, a, "admin@careops.local", "Admin!234567")

	resp := getWithSession(t, a, "/reports/new", cookie)
	body := readBody(t, resp)

	requiredKeys := []string{"date_from", "date_to", "resident_id", "user_id", "entity_type"}
	for _, k := range requiredKeys {
		if !strings.Contains(body, k) {
			t.Errorf("reports/new: parameter key %q missing from help text", k)
		}
	}
}

// ---------------------------------------------------------------------------
// Issue 7: README security/traceability statements
// ---------------------------------------------------------------------------

// TestREADME_IdempotencyStatementCoversAllOperations verifies that the README
// idempotency section mentions shift open/close and batch import — operations
// that were missing in the stale version.
func TestREADME_IdempotencyStatementCoversAllOperations(t *testing.T) {
	readmePath := filepath.Join(repoRoot(), "README.md")
	data, err := os.ReadFile(readmePath)
	if err != nil {
		t.Fatalf("read README.md: %v", err)
	}
	body := string(data)

	required := []string{
		"shift open",
		"shift open/close",
		"batch import",
	}
	for _, phrase := range required {
		if !strings.Contains(strings.ToLower(body), strings.ToLower(phrase)) {
			t.Errorf("README.md idempotency section: %q not mentioned", phrase)
		}
	}
}

// TestREADME_OCCStatementListsSpecificEntities verifies that the README OCC
// section names the specific entity types covered by row_version, not just a
// vague "write-likely entities" statement without detail.
func TestREADME_OCCStatementListsSpecificEntities(t *testing.T) {
	readmePath := filepath.Join(repoRoot(), "README.md")
	data, err := os.ReadFile(readmePath)
	if err != nil {
		t.Fatalf("read README.md: %v", err)
	}
	body := string(data)

	entities := []string{"payments", "service orders", "exam sessions", "config entries"}
	for _, e := range entities {
		if !strings.Contains(strings.ToLower(body), strings.ToLower(e)) {
			t.Errorf("README.md OCC section: entity %q not listed", e)
		}
	}
}

// TestREADME_IdempotencyKeyHeaderNamedCorrectly asserts that the README names
// the idempotency mechanism consistently (header name and form field).
func TestREADME_IdempotencyKeyHeaderNamedCorrectly(t *testing.T) {
	readmePath := filepath.Join(repoRoot(), "README.md")
	data, err := os.ReadFile(readmePath)
	if err != nil {
		t.Fatalf("read README.md: %v", err)
	}
	body := string(data)

	if !strings.Contains(body, "X-Idempotency-Key") {
		t.Error("README.md: should name the X-Idempotency-Key header")
	}
	if !strings.Contains(body, "_idempotency_key") {
		t.Error("README.md: should name the _idempotency_key form field")
	}
}

// TestREADME_NoStaleOCCClaims checks that the README does not contain any
// phrasing that was known to be wrong before the fix (claiming OCC only
// covered a single entity or omitting key write-likely entities).
func TestREADME_NoStaleOCCClaims(t *testing.T) {
	readmePath := filepath.Join(repoRoot(), "README.md")
	data, err := os.ReadFile(readmePath)
	if err != nil {
		t.Fatalf("read README.md: %v", err)
	}
	body := string(data)

	// The stale statement only covered "config_entries" without listing the others.
	stale := []string{
		"OCC only applies to config",
		"row_version on config entries only",
	}
	for _, s := range stale {
		if strings.Contains(strings.ToLower(body), strings.ToLower(s)) {
			t.Errorf("README.md: stale OCC claim %q must be removed", s)
		}
	}
}
