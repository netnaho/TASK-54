package api_tests

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestLogin_GetRendersForm(t *testing.T) {
	a := buildTestApp(t)
	req := httptest.NewRequest(http.MethodGet, "/login", nil)
	resp, err := a.Test(req, -1)
	if err != nil {
		t.Fatalf("GET /login: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestLogin_ValidCredentialsRedirectsToDashboard(t *testing.T) {
	a := buildTestApp(t)

	form := url.Values{}
	form.Set("email", "admin@careops.local")
	form.Set("password", "Admin!234567")

	req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := a.Test(req, -1)
	if err != nil {
		t.Fatalf("POST /login: %v", err)
	}

	// Expect redirect to /dashboard
	if resp.StatusCode != http.StatusFound && resp.StatusCode != http.StatusSeeOther {
		t.Errorf("expected 302/303 redirect, got %d", resp.StatusCode)
	}

	loc := resp.Header.Get("Location")
	if loc != "/dashboard" {
		t.Errorf("expected redirect to /dashboard, got %q", loc)
	}

	// Session cookie must be set
	found := false
	for _, c := range resp.Cookies() {
		if c.Name == "careops_session" {
			found = true
			if c.Value == "" {
				t.Error("session cookie value is empty")
			}
		}
	}
	if !found {
		t.Error("careops_session cookie not set after login")
	}
}

func TestLogin_InvalidCredentialsReRendersForm(t *testing.T) {
	a := buildTestApp(t)

	form := url.Values{}
	form.Set("email", "admin@careops.local")
	form.Set("password", "WRONGPASSWORD")

	req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := a.Test(req, -1)
	if err != nil {
		t.Fatalf("POST /login (bad creds): %v", err)
	}

	// Should re-render the login form (200), not redirect
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestLogin_UnknownEmailReturns200(t *testing.T) {
	a := buildTestApp(t)

	form := url.Values{}
	form.Set("email", "nobody@example.com")
	form.Set("password", "irrelevant")

	req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := a.Test(req, -1)
	if err != nil {
		t.Fatalf("POST /login (unknown): %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}
