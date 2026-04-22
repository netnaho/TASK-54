package api_tests

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

// protectedRoutes are all module index pages that require a valid session.
var protectedRoutes = []string{
	"/dashboard",
	"/users",
	"/beds",
	"/admissions",
	"/occupancy",
	"/service-delivery",
	"/exercise-library",
	"/exams",
	"/finance",
	"/reports",
	"/audit",
	"/jobs",
	"/config-versions",
	"/diagnostics",
}

func TestProtectedRoutes_UnauthenticatedRedirectsToLogin(t *testing.T) {
	a := buildTestApp(t)
	for _, path := range protectedRoutes {
		path := path
		t.Run(path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, path, nil)
			resp, err := a.Test(req, -1)
			if err != nil {
				t.Fatalf("GET %s: %v", path, err)
			}
			if resp.StatusCode != http.StatusFound && resp.StatusCode != http.StatusSeeOther {
				t.Errorf("%s: expected redirect (302/303), got %d", path, resp.StatusCode)
			}
			loc := resp.Header.Get("Location")
			if loc != "/login" {
				t.Errorf("%s: expected redirect to /login, got %q", path, loc)
			}
		})
	}
}

func TestProtectedRoutes_AuthenticatedReturns200(t *testing.T) {
	a := buildTestApp(t)

	// Log in as admin
	form := url.Values{}
	form.Set("email", "admin@careops.local")
	form.Set("password", "Admin!234567")

	loginReq := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(form.Encode()))
	loginReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	loginResp, err := a.Test(loginReq, -1)
	if err != nil {
		t.Fatalf("login: %v", err)
	}

	var sessionCookie string
	for _, c := range loginResp.Cookies() {
		if c.Name == "careops_session" {
			sessionCookie = c.Value
		}
	}
	if sessionCookie == "" {
		t.Fatal("no session cookie after login")
	}

	// expectedBodyMarker maps each protected path to a string that must appear
	// in the response body when the page renders correctly.
	expectedBodyMarker := map[string]string{
		"/dashboard":       "Operations Dashboard",
		"/users":           "Staff Users",
		"/beds":            "Bed Management",
		"/admissions":      "Admissions",
		"/occupancy":       "Occupancy",
		"/service-delivery": "Service Delivery",
		"/exercise-library": "Exercise Library",
		"/exams":           "Competency Exams",
		"/finance":         "Finance",
		"/reports":         "Reports",
		"/audit":           "Audit Log",
		"/jobs":            "Job History",
		"/config-versions": "Configuration",
		"/diagnostics":     "Diagnostics",
	}

	for _, path := range protectedRoutes {
		path := path
		t.Run(path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, path, nil)
			req.AddCookie(&http.Cookie{Name: "careops_session", Value: sessionCookie})
			resp, err := a.Test(req, -1)
			if err != nil {
				t.Fatalf("GET %s: %v", path, err)
			}
			if resp.StatusCode != http.StatusOK {
				t.Errorf("%s: expected 200, got %d", path, resp.StatusCode)
			}

			// Body content assertion: page must render module-specific text.
			bodyBytes, _ := io.ReadAll(resp.Body)
			body := string(bodyBytes)
			if marker, ok := expectedBodyMarker[path]; ok {
				if !strings.Contains(body, marker) {
					t.Errorf("%s: body missing expected marker %q", path, marker)
				}
			}

			// Every authenticated page must include the nav sidebar link.
			if !strings.Contains(body, `href="/dashboard"`) {
				t.Errorf("%s: body missing nav link to /dashboard (page may not have rendered layout)", path)
			}
		})
	}
}
