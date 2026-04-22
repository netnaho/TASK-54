package api_tests

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

// ── Config versions optimistic lock ───────────────────────────────────────────

// TestOptimisticLock_ConfigVersion_StaleRowVersionReturns409 gets a config
// entry, submits an update with the current row_version, then tries to submit
// again with the original (now stale) row_version — the second update should
// fail with 409 (conflict detected at the DB layer, surfaced as 409 by the handler).
func TestOptimisticLock_ConfigVersion_StaleRowVersionReturns409(t *testing.T) {
	a := buildTestApp(t)
	cookie := loginAs(t, a, "admin@careops.local", "Admin!234567")

	// Get index to find a config ID.
	indexResp := getWithSession(t, a, "/config-versions", cookie)
	indexBody, _ := io.ReadAll(indexResp.Body)
	indexStr := string(indexBody)

	editLinkPrefix := `href="/config-versions/`
	idx := strings.Index(indexStr, editLinkPrefix)
	if idx < 0 {
		t.Skip("no config entries in index — seeds may not have loaded")
	}
	start := idx + len(editLinkPrefix)
	end := strings.Index(indexStr[start:], "/edit")
	if end < 0 {
		t.Skip("no /edit link in config-versions index")
	}
	configID := indexStr[start : start+end]
	if strings.ContainsAny(configID, ` "'<>`) {
		t.Skipf("extracted configID looks wrong: %q", configID)
	}

	// Get the edit form to extract the current row_version.
	editResp := getWithSession(t, a, "/config-versions/"+configID+"/edit", cookie)
	editBody, _ := io.ReadAll(editResp.Body)
	editStr := string(editBody)

	rvPrefix := `name="row_version" value="`
	rvIdx := strings.Index(editStr, rvPrefix)
	if rvIdx < 0 {
		t.Skip("row_version not found in edit form")
	}
	rvStart := rvIdx + len(rvPrefix)
	rvEnd := strings.Index(editStr[rvStart:], `"`)
	originalRowVersion := editStr[rvStart : rvStart+rvEnd]

	// First update — should succeed with the current row_version.
	form1 := url.Values{}
	form1.Set("config_value", "updated-value-first")
	form1.Set("row_version", originalRowVersion)

	req1 := httptest.NewRequest(http.MethodPost, "/config-versions/"+configID, strings.NewReader(form1.Encode()))
	req1.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req1.AddCookie(&http.Cookie{Name: "careops_session", Value: cookie})

	resp1, err := a.Test(req1, -1)
	if err != nil {
		t.Fatalf("first update: %v", err)
	}
	if resp1.StatusCode != http.StatusFound && resp1.StatusCode != http.StatusSeeOther {
		body, _ := io.ReadAll(resp1.Body)
		t.Fatalf("first update: want redirect, got %d — %s", resp1.StatusCode, string(body)[:min(200, len(body))])
	}

	// Second update with the original (now stale) row_version — must fail with 409.
	form2 := url.Values{}
	form2.Set("config_value", "updated-value-stale")
	form2.Set("row_version", originalRowVersion) // stale — version was bumped by first update

	req2 := httptest.NewRequest(http.MethodPost, "/config-versions/"+configID, strings.NewReader(form2.Encode()))
	req2.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req2.AddCookie(&http.Cookie{Name: "careops_session", Value: cookie})

	resp2, err := a.Test(req2, -1)
	if err != nil {
		t.Fatalf("stale update: %v", err)
	}
	// The config_versions handler detects apperr.ErrConflict and returns 409.
	if resp2.StatusCode != http.StatusConflict {
		body, _ := io.ReadAll(resp2.Body)
		t.Errorf("stale row_version update: want 409, got %d — %s", resp2.StatusCode, string(body)[:min(300, len(body))])
	}
}

// ── Exam session publish optimistic lock ──────────────────────────────────────

// TestOptimisticLock_ExamSessionPublish_WrongRowVersion creates an exam
// template and session, then tries to publish with a wrong row_version — should
// return 400 or 409.
func TestOptimisticLock_ExamSessionPublish_WrongRowVersion(t *testing.T) {
	a := buildTestApp(t)
	cookie := loginAs(t, a, "admin@careops.local", "Admin!234567")

	// Create an exam template first.
	tmplForm := url.Values{}
	tmplForm.Set("title", "OL Test Template")
	tmplForm.Set("description", "For optimistic lock testing")
	tmplForm.Set("role_scope", "nurse")
	tmplForm.Set("passing_score", "80")
	tmplForm.Set("duration_minutes", "60")
	tmplForm.Set("window_start", "08:00")
	tmplForm.Set("window_end", "17:00")

	tmplReq := httptest.NewRequest(http.MethodPost, "/exams/templates", strings.NewReader(tmplForm.Encode()))
	tmplReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	tmplReq.AddCookie(&http.Cookie{Name: "careops_session", Value: cookie})
	tmplResp, err := a.Test(tmplReq, -1)
	if err != nil {
		t.Fatalf("create template: %v", err)
	}
	if tmplResp.StatusCode != http.StatusFound && tmplResp.StatusCode != http.StatusSeeOther {
		body, _ := io.ReadAll(tmplResp.Body)
		t.Fatalf("create template: want redirect, got %d — %s", tmplResp.StatusCode, string(body)[:min(300, len(body))])
	}

	// Extract the template ID from the redirect location.
	tmplLoc := tmplResp.Header.Get("Location")
	// Location should be /exams/templates/<id>?flash=...
	tmplIDStart := strings.LastIndex(tmplLoc, "/") + 1
	// Strip query string if present.
	tmplIDWithQuery := tmplLoc[tmplIDStart:]
	templateID := strings.SplitN(tmplIDWithQuery, "?", 2)[0]
	if templateID == "" {
		t.Skip("could not extract template ID from redirect")
	}

	// Generate a session from the template.
	sessForm := url.Values{}
	sessForm.Set("template_id", templateID)
	sessForm.Set("planned_start", "2026-12-01T09:00")
	sessForm.Set("room", "Room A")
	sessForm.Set("proctor_id", "")

	sessReq := httptest.NewRequest(http.MethodPost, "/exams/sessions", strings.NewReader(sessForm.Encode()))
	sessReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	sessReq.AddCookie(&http.Cookie{Name: "careops_session", Value: cookie})
	sessResp, err := a.Test(sessReq, -1)
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	if sessResp.StatusCode != http.StatusFound && sessResp.StatusCode != http.StatusSeeOther {
		body, _ := io.ReadAll(sessResp.Body)
		t.Fatalf("create session: want redirect, got %d — %s", sessResp.StatusCode, string(body)[:min(300, len(body))])
	}

	// Extract the session ID from the redirect location.
	sessLoc := sessResp.Header.Get("Location")
	sessIDStart := strings.LastIndex(sessLoc, "/") + 1
	sessIDWithQuery := sessLoc[sessIDStart:]
	sessionID := strings.SplitN(sessIDWithQuery, "?", 2)[0]
	if sessionID == "" {
		t.Skip("could not extract session ID from redirect")
	}

	// Publish with a deliberately wrong row_version (9999) — must fail.
	pubForm := url.Values{}
	pubForm.Set("row_version", "9999")

	pubReq := httptest.NewRequest(http.MethodPost, "/exams/sessions/"+sessionID+"/publish", strings.NewReader(pubForm.Encode()))
	pubReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	pubReq.AddCookie(&http.Cookie{Name: "careops_session", Value: cookie})

	pubResp, err := a.Test(pubReq, -1)
	if err != nil {
		t.Fatalf("publish with wrong row_version: %v", err)
	}
	// ExamHandler.SessionPublish wraps ErrVersionConflict as fiber.StatusConflict (409),
	// or the error handler may render a 409 page.
	if pubResp.StatusCode != http.StatusBadRequest && pubResp.StatusCode != http.StatusConflict {
		body, _ := io.ReadAll(pubResp.Body)
		t.Errorf("publish wrong row_version: want 400 or 409, got %d — %s", pubResp.StatusCode, string(body)[:min(300, len(body))])
	}
}
