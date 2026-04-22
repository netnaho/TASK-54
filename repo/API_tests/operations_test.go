package api_tests

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

// loginSession logs in as admin and returns the session cookie value.
func loginSession(t *testing.T) (interface{ Test(*http.Request, ...int) (*http.Response, error) }, string) {
	t.Helper()
	a := buildTestApp(t)

	form := url.Values{}
	form.Set("email", "admin@careops.local")
	form.Set("password", "Admin!234567")

	req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := a.Test(req, -1)
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	for _, c := range resp.Cookies() {
		if c.Name == "careops_session" {
			return a, c.Value
		}
	}
	t.Fatal("no session cookie after login")
	return nil, ""
}

func authedGet(t *testing.T, a interface{ Test(*http.Request, ...int) (*http.Response, error) }, path, sessionCookie string) *http.Response {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	req.AddCookie(&http.Cookie{Name: "careops_session", Value: sessionCookie})
	resp, err := a.Test(req, -1)
	if err != nil {
		t.Fatalf("GET %s: %v", path, err)
	}
	return resp
}

func authedPost(t *testing.T, a interface{ Test(*http.Request, ...int) (*http.Response, error) }, path, sessionCookie string, body url.Values) *http.Response {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(&http.Cookie{Name: "careops_session", Value: sessionCookie})
	resp, err := a.Test(req, -1)
	if err != nil {
		t.Fatalf("POST %s: %v", path, err)
	}
	return resp
}

// ── Occupancy ────────────────────────────────────────────────────────────────

func TestOccupancy_Returns200(t *testing.T) {
	a, sess := loginSession(t)
	resp := authedGet(t, a, "/occupancy", sess)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("/occupancy: expected 200, got %d", resp.StatusCode)
	}
}

func TestOccupancy_ContainsLiveData(t *testing.T) {
	a, sess := loginSession(t)
	resp := authedGet(t, a, "/occupancy", sess)
	body, _ := io.ReadAll(resp.Body)
	// Page must contain the live occupancy section heading
	if !strings.Contains(string(body), "Live Occupancy") {
		t.Error("/occupancy: response missing 'Live Occupancy' section")
	}
}

func TestOccupancy_HTMX_PartialNotAvailable(t *testing.T) {
	// Occupancy has no partial — full page only; HTMX request still returns 200
	a, sess := loginSession(t)
	req := httptest.NewRequest(http.MethodGet, "/occupancy", nil)
	req.AddCookie(&http.Cookie{Name: "careops_session", Value: sess})
	req.Header.Set("HX-Request", "true")
	resp, err := a.Test(req, -1)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

// ── Service Delivery ─────────────────────────────────────────────────────────

func TestServiceDelivery_IndexReturns200(t *testing.T) {
	a, sess := loginSession(t)
	resp := authedGet(t, a, "/service-delivery", sess)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("/service-delivery: expected 200, got %d", resp.StatusCode)
	}
}

func TestServiceDelivery_MetricsRendered(t *testing.T) {
	a, sess := loginSession(t)
	resp := authedGet(t, a, "/service-delivery", sess)
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "Completion Rate") {
		t.Error("service delivery page missing Completion Rate metric")
	}
	if !strings.Contains(string(body), "On-Time Rate") {
		t.Error("service delivery page missing On-Time Rate metric")
	}
}

func TestServiceDelivery_HTMXPartial(t *testing.T) {
	a, sess := loginSession(t)
	req := httptest.NewRequest(http.MethodGet, "/service-delivery", nil)
	req.AddCookie(&http.Cookie{Name: "careops_session", Value: sess})
	req.Header.Set("HX-Request", "true")
	resp, err := a.Test(req, -1)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("HTMX partial: expected 200, got %d", resp.StatusCode)
	}
	// Partial should not include full HTML layout
	body, _ := io.ReadAll(resp.Body)
	if strings.Contains(string(body), "<!DOCTYPE") {
		t.Error("HTMX partial should not contain full HTML layout")
	}
}

func TestServiceDelivery_DateRangeFilter(t *testing.T) {
	a, sess := loginSession(t)
	resp := authedGet(t, a, "/service-delivery?date_from=2024-01-01&date_to=2024-12-31", sess)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("date range filter: expected 200, got %d", resp.StatusCode)
	}
}

// ── Alert Create ──────────────────────────────────────────────────────────────

func TestAlertCreate_FormRenders(t *testing.T) {
	a, sess := loginSession(t)
	resp := authedGet(t, a, "/service-delivery/alerts/new", sess)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("alert form: expected 200, got %d", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "Alert") {
		t.Error("alert form: missing alert heading")
	}
}

func TestAlertCreate_EmptyFieldsReturn422(t *testing.T) {
	a, sess := loginSession(t)
	form := url.Values{}
	form.Set("resident_id", "")
	form.Set("alert_type", "")
	form.Set("severity", "")
	form.Set("message", "")
	resp := authedPost(t, a, "/service-delivery/alerts", sess, form)
	if resp.StatusCode != http.StatusBadRequest && resp.StatusCode != http.StatusUnprocessableEntity {
		t.Errorf("empty alert: expected 400 or 422, got %d", resp.StatusCode)
	}
}

func TestAlertCreate_ValidDataRedirects(t *testing.T) {
	a, sess := loginSession(t)
	// Fetch a valid resident ID from the seeded data
	form := url.Values{}
	form.Set("resident_id", "res-001") // seeded resident ID
	form.Set("alert_type", "fall_risk")
	form.Set("severity", "high")
	form.Set("message", "Patient exhibited unsteady gait during therapy")
	resp := authedPost(t, a, "/service-delivery/alerts", sess, form)
	// Expect redirect (302/303) after successful create
	if resp.StatusCode != http.StatusFound && resp.StatusCode != http.StatusSeeOther {
		t.Errorf("valid alert create: expected redirect, got %d", resp.StatusCode)
	}
}

func TestAlertCreate_Idempotency(t *testing.T) {
	a, sess := loginSession(t)
	form := url.Values{}
	form.Set("resident_id", "res-001")
	form.Set("alert_type", "fall_risk")
	form.Set("severity", "medium")
	form.Set("message", "Idempotency test alert")
	idempKey := "test-idem-alert-001"
	form.Set("_idempotency_key", idempKey)

	resp1 := authedPost(t, a, "/service-delivery/alerts", sess, form)
	resp2 := authedPost(t, a, "/service-delivery/alerts", sess, form)

	// Both calls must redirect; 200 is not acceptable (would indicate re-rendered form).
	for i, r := range []*http.Response{resp1, resp2} {
		if r.StatusCode != http.StatusFound && r.StatusCode != http.StatusSeeOther {
			t.Errorf("alert idempotency request %d: want redirect, got %d", i+1, r.StatusCode)
		}
	}
}

// ── Checkpoint Create ─────────────────────────────────────────────────────────

func TestCheckpointCreate_FormRenders(t *testing.T) {
	a, sess := loginSession(t)
	resp := authedGet(t, a, "/service-delivery/checkpoints/new", sess)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("checkpoint form: expected 200, got %d", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "Care Checkpoint") {
		t.Error("checkpoint form: missing heading")
	}
}

func TestCheckpointCreate_EmptyFieldsReturn422(t *testing.T) {
	a, sess := loginSession(t)
	form := url.Values{}
	form.Set("resident_id", "")
	form.Set("checkpoint_type", "")
	resp := authedPost(t, a, "/service-delivery/checkpoints", sess, form)
	if resp.StatusCode != http.StatusBadRequest && resp.StatusCode != http.StatusUnprocessableEntity {
		t.Errorf("empty checkpoint: expected 400 or 422, got %d", resp.StatusCode)
	}
}

func TestCheckpointCreate_ValidDataRedirects(t *testing.T) {
	a, sess := loginSession(t)
	form := url.Values{}
	form.Set("resident_id", "res-001")
	form.Set("checkpoint_type", "mobility")
	form.Set("score", "4")
	form.Set("notes", "Patient progressing well with walker")
	resp := authedPost(t, a, "/service-delivery/checkpoints", sess, form)
	if resp.StatusCode != http.StatusFound && resp.StatusCode != http.StatusSeeOther {
		t.Errorf("valid checkpoint: expected redirect, got %d", resp.StatusCode)
	}
}

// ── Exercise Library ──────────────────────────────────────────────────────────

func TestExerciseLibrary_IndexReturns200(t *testing.T) {
	a, sess := loginSession(t)
	resp := authedGet(t, a, "/exercise-library", sess)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("/exercise-library: expected 200, got %d", resp.StatusCode)
	}
}

func TestExerciseLibrary_FilterByCategory(t *testing.T) {
	a, sess := loginSession(t)
	resp := authedGet(t, a, "/exercise-library?category=strength", sess)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("filter by category: expected 200, got %d", resp.StatusCode)
	}
}

func TestExerciseLibrary_FilterByTag(t *testing.T) {
	a, sess := loginSession(t)
	resp := authedGet(t, a, "/exercise-library?tag=upper-body", sess)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("filter by tag: expected 200, got %d", resp.StatusCode)
	}
}

func TestExerciseLibrary_SearchFilter(t *testing.T) {
	a, sess := loginSession(t)
	resp := authedGet(t, a, "/exercise-library?q=squat", sess)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("search filter: expected 200, got %d", resp.StatusCode)
	}
}

func TestExerciseLibrary_HTMXPartial(t *testing.T) {
	a, sess := loginSession(t)
	req := httptest.NewRequest(http.MethodGet, "/exercise-library", nil)
	req.AddCookie(&http.Cookie{Name: "careops_session", Value: sess})
	req.Header.Set("HX-Request", "true")
	resp, err := a.Test(req, -1)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("exercise HTMX partial: expected 200, got %d", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	if strings.Contains(string(body), "<!DOCTYPE") {
		t.Error("exercise HTMX partial should not include full layout")
	}
}

func TestExerciseLibrary_Detail(t *testing.T) {
	a, sess := loginSession(t)
	resp := authedGet(t, a, "/exercise-library/ex-001", sess)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("exercise detail: expected 200, got %d", resp.StatusCode)
	}
}

func TestExerciseLibrary_Detail_NotFound(t *testing.T) {
	a, sess := loginSession(t)
	resp := authedGet(t, a, "/exercise-library/nonexistent-id-xyz", sess)
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("exercise not found: expected 404, got %d", resp.StatusCode)
	}
}

func TestExerciseLibrary_FavoriteToggle(t *testing.T) {
	a, sess := loginSession(t)
	form := url.Values{}
	resp := authedPost(t, a, "/exercise-library/ex-001/favorite", sess, form)
	// Should redirect or return 200 (HTMX swap)
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusFound && resp.StatusCode != http.StatusSeeOther {
		t.Errorf("favorite toggle: unexpected status %d", resp.StatusCode)
	}
}

func TestExerciseLibrary_FavoriteToggle_Unauthenticated(t *testing.T) {
	a := buildTestApp(t)
	form := url.Values{}
	req := httptest.NewRequest(http.MethodPost, "/exercise-library/ex-001/favorite", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := a.Test(req, -1)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusFound && resp.StatusCode != http.StatusSeeOther {
		t.Errorf("unauthenticated favorite toggle: expected redirect to login, got %d", resp.StatusCode)
	}
}

// ── Beds ──────────────────────────────────────────────────────────────────────

func TestBeds_IndexReturns200(t *testing.T) {
	a, sess := loginSession(t)
	resp := authedGet(t, a, "/beds", sess)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("/beds: expected 200, got %d", resp.StatusCode)
	}
}

func TestBeds_WardFilter(t *testing.T) {
	a, sess := loginSession(t)
	resp := authedGet(t, a, "/beds?ward=North+Wing", sess)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("beds ward filter: expected 200, got %d", resp.StatusCode)
	}
}

func TestBeds_Detail(t *testing.T) {
	a, sess := loginSession(t)
	resp := authedGet(t, a, "/beds/bed-001", sess)
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNotFound {
		t.Errorf("bed detail: expected 200 or 404, got %d", resp.StatusCode)
	}
}

func TestBeds_Detail_Unauthenticated(t *testing.T) {
	a := buildTestApp(t)
	req := httptest.NewRequest(http.MethodGet, "/beds/bed-001", nil)
	resp, err := a.Test(req, -1)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusFound && resp.StatusCode != http.StatusSeeOther {
		t.Errorf("unauthenticated bed detail: expected redirect, got %d", resp.StatusCode)
	}
}

// ── Admissions / Residents ────────────────────────────────────────────────────

func TestAdmissions_IndexReturns200(t *testing.T) {
	a, sess := loginSession(t)
	resp := authedGet(t, a, "/admissions", sess)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("/admissions: expected 200, got %d", resp.StatusCode)
	}
}

func TestAdmissions_FilterByCareLevel(t *testing.T) {
	a, sess := loginSession(t)
	resp := authedGet(t, a, "/admissions?care_level=skilled", sess)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("admissions care_level filter: expected 200, got %d", resp.StatusCode)
	}
}

func TestAdmissions_FilterByStatus(t *testing.T) {
	a, sess := loginSession(t)
	resp := authedGet(t, a, "/admissions?status=active", sess)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("admissions status filter: expected 200, got %d", resp.StatusCode)
	}
}

func TestAdmissions_ResidentDetail(t *testing.T) {
	a, sess := loginSession(t)
	resp := authedGet(t, a, "/admissions/res-001", sess)
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNotFound {
		t.Errorf("resident detail: expected 200 or 404, got %d", resp.StatusCode)
	}
}

func TestAdmissions_ResidentDetail_Unauthenticated(t *testing.T) {
	a := buildTestApp(t)
	req := httptest.NewRequest(http.MethodGet, "/admissions/res-001", nil)
	resp, err := a.Test(req, -1)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusFound && resp.StatusCode != http.StatusSeeOther {
		t.Errorf("unauthenticated resident detail: expected redirect, got %d", resp.StatusCode)
	}
}

// ── Media Serving ─────────────────────────────────────────────────────────────

func TestMedia_UnauthenticatedRejected(t *testing.T) {
	a := buildTestApp(t)
	req := httptest.NewRequest(http.MethodGet, "/media/test.jpg", nil)
	resp, err := a.Test(req, -1)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusFound && resp.StatusCode != http.StatusSeeOther {
		t.Errorf("unauthenticated media: expected redirect, got %d", resp.StatusCode)
	}
}

func TestMedia_PathTraversalRejected(t *testing.T) {
	a, sess := loginSession(t)
	// Encoded path traversal: ../etc/passwd
	for _, payload := range []string{"../etc/passwd", "..%2Fetc%2Fpasswd"} {
		req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/media/%s", payload), nil)
		req.AddCookie(&http.Cookie{Name: "careops_session", Value: sess})
		resp, err := a.Test(req, -1)
		if err != nil {
			t.Fatalf("media traversal %q: %v", payload, err)
		}
		if resp.StatusCode == http.StatusOK {
			t.Errorf("media traversal %q: expected non-200, got %d", payload, resp.StatusCode)
		}
	}
}

// ── Unauthorized role access ──────────────────────────────────────────────────

func TestUnauthorized_AlertCreateRequiresAuth(t *testing.T) {
	a := buildTestApp(t)
	req := httptest.NewRequest(http.MethodGet, "/service-delivery/alerts/new", nil)
	resp, err := a.Test(req, -1)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusFound && resp.StatusCode != http.StatusSeeOther {
		t.Errorf("unauthenticated alert form: expected redirect, got %d", resp.StatusCode)
	}
}

func TestUnauthorized_CheckpointCreateRequiresAuth(t *testing.T) {
	a := buildTestApp(t)
	req := httptest.NewRequest(http.MethodGet, "/service-delivery/checkpoints/new", nil)
	resp, err := a.Test(req, -1)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusFound && resp.StatusCode != http.StatusSeeOther {
		t.Errorf("unauthenticated checkpoint form: expected redirect, got %d", resp.StatusCode)
	}
}
