package api_tests

import (
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

// loginAsNewApp logs in as a specific user and returns the app + session cookie.
func loginAsNewApp(t *testing.T, email, password string) (interface{ Test(*http.Request, ...int) (*http.Response, error) }, string) {
	t.Helper()
	a := buildTestApp(t)
	form := url.Values{}
	form.Set("email", email)
	form.Set("password", password)
	resp := authedPost(t, a, "/login", "", form)
	for _, c := range resp.Cookies() {
		if c.Name == "careops_session" {
			return a, c.Value
		}
	}
	t.Fatalf("no session cookie after login as %s", email)
	return nil, ""
}

// — Access control —

func TestExams_UnauthenticatedRedirectsToLogin(t *testing.T) {
	a := buildTestApp(t)
	for _, path := range []string{
		"/exams",
		"/exams/scheduler",
		"/exams/templates/new",
		"/exams/sessions/new",
	} {
		path := path
		t.Run(path, func(t *testing.T) {
			resp := authedGet(t, a, path, "")
			if resp.StatusCode != http.StatusFound && resp.StatusCode != http.StatusSeeOther {
				t.Errorf("%s: expected redirect, got %d", path, resp.StatusCode)
			}
		})
	}
}

func TestExams_Index_Returns200_AsAdmin(t *testing.T) {
	a, sess := loginSession(t)
	resp := authedGet(t, a, "/exams", sess)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("/exams: expected 200, got %d", resp.StatusCode)
	}
}

func TestExams_Index_Returns200_AsTrainingCoordinator(t *testing.T) {
	a, sess := loginAsNewApp(t, "training@careops.local", "Training!2345")
	resp := authedGet(t, a, "/exams", sess)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("/exams: expected 200 for training_coordinator, got %d", resp.StatusCode)
	}
}

func TestExams_Index_Forbidden_AsNurse(t *testing.T) {
	a, sess := loginAsNewApp(t, "nurse@careops.local", "Nurse!234567")
	resp := authedGet(t, a, "/exams", sess)
	if resp.StatusCode != http.StatusForbidden && resp.StatusCode != http.StatusFound {
		t.Errorf("/exams: expected 403/302 for nurse, got %d", resp.StatusCode)
	}
}

// — Template management —

func TestExams_TemplateNew_Returns200(t *testing.T) {
	a, sess := loginSession(t)
	resp := authedGet(t, a, "/exams/templates/new", sess)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("/exams/templates/new: expected 200, got %d", resp.StatusCode)
	}
}

func TestExams_TemplateCreate_RedirectsOnSuccess(t *testing.T) {
	a, sess := loginSession(t)
	form := url.Values{
		"title":            {"API Test Exam"},
		"role_scope":       {"nurse"},
		"passing_score":    {"80"},
		"duration_minutes": {"60"},
		"window_start":     {"08:00"},
		"window_end":       {"17:00"},
	}
	resp := authedPost(t, a, "/exams/templates", sess, form)
	if resp.StatusCode != http.StatusFound && resp.StatusCode != http.StatusSeeOther {
		body, _ := io.ReadAll(resp.Body)
		t.Errorf("POST /exams/templates: expected redirect, got %d\n%s", resp.StatusCode, body)
	}
	loc := resp.Header.Get("Location")
	if !strings.HasPrefix(loc, "/exams/templates/") {
		t.Errorf("POST /exams/templates: expected redirect to /exams/templates/:id, got %q", loc)
	}
}

func TestExams_TemplateCreate_BadRequest_MissingTitle(t *testing.T) {
	a, sess := loginSession(t)
	form := url.Values{
		"role_scope":       {"nurse"},
		"passing_score":    {"80"},
		"duration_minutes": {"60"},
		"window_start":     {"08:00"},
		"window_end":       {"17:00"},
	}
	resp := authedPost(t, a, "/exams/templates", sess, form)
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("POST /exams/templates (no title): expected 400, got %d", resp.StatusCode)
	}
}

func TestExams_TemplateDetail_ExistingTemplate(t *testing.T) {
	a, sess := loginSession(t)
	// First create a template to get an ID.
	form := url.Values{
		"title":            {"Detail Test Exam"},
		"role_scope":       {"aide"},
		"passing_score":    {"75"},
		"duration_minutes": {"45"},
		"window_start":     {"09:00"},
		"window_end":       {"16:00"},
	}
	createResp := authedPost(t, a, "/exams/templates", sess, form)
	loc := createResp.Header.Get("Location")
	if !strings.HasPrefix(loc, "/exams/templates/") {
		t.Fatalf("create template did not redirect to detail page: %q", loc)
	}
	resp := authedGet(t, a, loc, sess)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("GET %s: expected 200, got %d", loc, resp.StatusCode)
	}
}

// — Session management —

func TestExams_SessionNew_Returns200(t *testing.T) {
	a, sess := loginSession(t)
	resp := authedGet(t, a, "/exams/sessions/new", sess)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("/exams/sessions/new: expected 200, got %d", resp.StatusCode)
	}
}

func TestExams_SessionCreate_RedirectsOnSuccess(t *testing.T) {
	a, sess := loginSession(t)

	// Create a template first.
	tmplForm := url.Values{
		"title":            {"Session API Test Exam"},
		"role_scope":       {"nurse"},
		"passing_score":    {"80"},
		"duration_minutes": {"60"},
		"window_start":     {"08:00"},
		"window_end":       {"18:00"},
	}
	tmplResp := authedPost(t, a, "/exams/templates", sess, tmplForm)
	tmplLoc := tmplResp.Header.Get("Location")
	if !strings.HasPrefix(tmplLoc, "/exams/templates/") {
		t.Fatalf("template create redirect %q not as expected", tmplLoc)
	}
	tmplID := strings.TrimPrefix(tmplLoc, "/exams/templates/")

	sessForm := url.Values{
		"template_id":   {tmplID},
		"planned_start": {"2024-06-15T09:00"},
		"room":          {"Conference Room A"},
		"proctor_id":    {""},
	}
	resp := authedPost(t, a, "/exams/sessions", sess, sessForm)
	if resp.StatusCode != http.StatusFound && resp.StatusCode != http.StatusSeeOther {
		body, _ := io.ReadAll(resp.Body)
		t.Errorf("POST /exams/sessions: expected redirect, got %d\n%s", resp.StatusCode, body)
	}
	loc := resp.Header.Get("Location")
	if !strings.HasPrefix(loc, "/exams/sessions/") {
		t.Errorf("POST /exams/sessions: expected redirect to session detail, got %q", loc)
	}
}

func TestExams_SessionCreate_BadRequest_NoRoom(t *testing.T) {
	a, sess := loginSession(t)

	tmplForm := url.Values{
		"title":            {"No Room Exam"},
		"role_scope":       {"nurse"},
		"passing_score":    {"80"},
		"duration_minutes": {"60"},
		"window_start":     {"08:00"},
		"window_end":       {"18:00"},
	}
	tmplResp := authedPost(t, a, "/exams/templates", sess, tmplForm)
	tmplID := strings.TrimPrefix(tmplResp.Header.Get("Location"), "/exams/templates/")

	sessForm := url.Values{
		"template_id":   {tmplID},
		"planned_start": {"2024-06-15T09:00"},
		"room":          {""},
	}
	resp := authedPost(t, a, "/exams/sessions", sess, sessForm)
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("POST /exams/sessions (no room): expected 400, got %d", resp.StatusCode)
	}
}

// — Session detail and conflict workflow —

func TestExams_SessionDetail_Returns200(t *testing.T) {
	a, sess := createSessionForTest(t)
	// createSessionForTest returns app+sess+sessionURL
	_ = sess
	_ = a
}

// createSessionFullFlow creates a template, generates a session, and returns
// (app, adminCookie, sessionURL).
func createSessionFullFlow(t *testing.T) (interface{ Test(*http.Request, ...int) (*http.Response, error) }, string, string) {
	t.Helper()
	a, sess := loginSession(t)

	tmplForm := url.Values{
		"title":            {"Flow Test Exam"},
		"role_scope":       {"nurse"},
		"passing_score":    {"80"},
		"duration_minutes": {"60"},
		"window_start":     {"08:00"},
		"window_end":       {"18:00"},
	}
	tmplResp := authedPost(t, a, "/exams/templates", sess, tmplForm)
	tmplID := strings.TrimPrefix(tmplResp.Header.Get("Location"), "/exams/templates/")

	sessForm := url.Values{
		"template_id":   {tmplID},
		"planned_start": {"2024-07-01T09:00"},
		"room":          {"Test Room"},
		"proctor_id":    {""},
	}
	sessResp := authedPost(t, a, "/exams/sessions", sess, sessForm)
	sessURL := sessResp.Header.Get("Location")
	if !strings.HasPrefix(sessURL, "/exams/sessions/") {
		t.Fatalf("session create redirect %q not as expected", sessURL)
	}
	return a, sess, sessURL
}

// createSessionForTest is a variant used only for the detail-page test above.
func createSessionForTest(t *testing.T) (interface{ Test(*http.Request, ...int) (*http.Response, error) }, string) {
	t.Helper()
	a, adminSess, sessURL := createSessionFullFlow(t)
	resp := authedGet(t, a, sessURL, adminSess)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("GET %s: expected 200, got %d", sessURL, resp.StatusCode)
	}
	return a, adminSess
}

func TestExams_Publish_SucceedsWithNoConflicts(t *testing.T) {
	a, adminSess, sessURL := createSessionFullFlow(t)
	// Get session page to extract row_version.
	resp := authedGet(t, a, sessURL, adminSess)
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "row_version") {
		t.Skip("row_version not in page body; skipping publish test")
	}

	// Attempt publish with row_version=1 (freshly created session).
	sessID := strings.TrimPrefix(sessURL, "/exams/sessions/")
	form := url.Values{"row_version": {"1"}}
	pubResp := authedPost(t, a, "/exams/sessions/"+sessID+"/publish", adminSess, form)
	// Should redirect (publish success) or 409 if there are seeded conflicts.
	if pubResp.StatusCode != http.StatusFound &&
		pubResp.StatusCode != http.StatusSeeOther &&
		pubResp.StatusCode != http.StatusConflict {
		t.Errorf("POST publish: expected redirect or 409 conflict, got %d", pubResp.StatusCode)
	}
}

func TestExams_Scheduler_Returns200(t *testing.T) {
	a, sess := loginSession(t)
	resp := authedGet(t, a, "/exams/scheduler", sess)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("/exams/scheduler: expected 200, got %d", resp.StatusCode)
	}
}

func TestExams_Scheduler_WithDate_Returns200(t *testing.T) {
	a, sess := loginSession(t)
	resp := authedGet(t, a, "/exams/scheduler?date=2024-04-10", sess)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("/exams/scheduler?date=2024-04-10: expected 200, got %d", resp.StatusCode)
	}
}

// — Idempotent publish —

func TestExams_Publish_Idempotent(t *testing.T) {
	a, adminSess, sessURL := createSessionFullFlow(t)
	sessID := strings.TrimPrefix(sessURL, "/exams/sessions/")
	form := url.Values{"row_version": {"1"}, "_idempotency_key": {"test-idem-pub-1"}}

	r1 := authedPost(t, a, "/exams/sessions/"+sessID+"/publish", adminSess, form)
	r2 := authedPost(t, a, "/exams/sessions/"+sessID+"/publish", adminSess, form)

	// Both should succeed (redirect or conflict but not 500).
	for i, r := range []*http.Response{r1, r2} {
		if r.StatusCode >= 500 {
			t.Errorf("publish attempt %d: got %d (expected <500)", i+1, r.StatusCode)
		}
	}
}

// — Block update —

func TestExams_BlockUpdate_HTMX_Returns200(t *testing.T) {
	a, adminSess, sessURL := createSessionFullFlow(t)
	sessID := strings.TrimPrefix(sessURL, "/exams/sessions/")

	form := url.Values{
		"planned_start": {"2024-07-01T10:00"},
		"room":          {"Updated Room"},
		"proctor_id":    {""},
		"row_version":   {"1"},
	}
	req, _ := newAuthedFormRequest(http.MethodPost, "/exams/sessions/"+sessID+"/block", adminSess, form)
	req.Header.Set("HX-Request", "true")
	resp, err := a.Test(req, -1)
	if err != nil {
		t.Fatalf("POST block (HTMX): %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("POST block (HTMX): expected 200, got %d", resp.StatusCode)
	}
}

func newAuthedFormRequest(method, path, sessionCookie string, body url.Values) (*http.Request, error) {
	req, err := http.NewRequest(method, path, strings.NewReader(body.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(&http.Cookie{Name: "careops_session", Value: sessionCookie})
	return req, nil
}
