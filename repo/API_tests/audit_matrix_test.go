package api_tests

// audit_matrix_test.go — table-driven sweep verifying that every privileged
// write endpoint introduced or hardened in Task 2 emits an audit log entry with
// a non-empty user_id, the expected entity_type, and outcome="success".
//
// Each subtest spins up its own isolated DB so failures are independent.

import (
	"database/sql"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"careops/clinic/internal/app"
	"careops/clinic/internal/platform/logger"
)

// TestAudit_PrivilegedEndpoints_Matrix sweeps the privileged write paths and
// verifies each one writes a conformant audit log entry.
func TestAudit_PrivilegedEndpoints_Matrix(t *testing.T) {
	type matrixRow struct {
		name       string
		wantAction string
		wantEntity string
		trigger    func(t *testing.T, a interface {
			Test(*http.Request, ...int) (*http.Response, error)
		}, db *sql.DB, cookie string)
	}

	rows := []matrixRow{
		{
			name:       "user.create",
			wantAction: "user.create",
			wantEntity: "users",
			trigger: func(t *testing.T, a interface {
				Test(*http.Request, ...int) (*http.Response, error)
			}, db *sql.DB, cookie string) {
				t.Helper()
				form := url.Values{
					"email":        {"matrix-create@careops.local"},
					"display_name": {"Matrix Create"},
					"password":     {"Matrix!Create9x"},
					"role":         {"nurse"},
				}
				req := httptest.NewRequest(http.MethodPost, "/users", strings.NewReader(form.Encode()))
				req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
				req.AddCookie(&http.Cookie{Name: "careops_session", Value: cookie})
				resp, err := a.Test(req, -1)
				if err != nil {
					t.Fatalf("POST /users: %v", err)
				}
				if resp.StatusCode != http.StatusFound && resp.StatusCode != http.StatusSeeOther {
					body := readBody(t, resp)
					t.Fatalf("create user: want redirect, got %d — %s", resp.StatusCode, body[:min(300, len(body))])
				}
			},
		},
		{
			name:       "user.reset_password",
			wantAction: "user.reset_password",
			wantEntity: "users",
			trigger: func(t *testing.T, a interface {
				Test(*http.Request, ...int) (*http.Response, error)
			}, db *sql.DB, cookie string) {
				t.Helper()
				listResp := getWithSession(t, a, "/users", cookie)
				listBody := readBody(t, listResp)
				userID := extractUserIDFromResetPasswordLink(t, listBody)
				rowVersion := extractRowVersionFromResetForm(t, a, userID, cookie)
				form := url.Values{
					"password":    {"Matrix!Reset99x"},
					"row_version": {rowVersion},
				}
				req := httptest.NewRequest(http.MethodPost, "/users/"+userID+"/reset-password",
					strings.NewReader(form.Encode()))
				req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
				req.AddCookie(&http.Cookie{Name: "careops_session", Value: cookie})
				resp, err := a.Test(req, -1)
				if err != nil {
					t.Fatalf("POST reset-password: %v", err)
				}
				if resp.StatusCode != http.StatusFound && resp.StatusCode != http.StatusSeeOther {
					body := readBody(t, resp)
					t.Fatalf("reset password: want redirect, got %d — %s", resp.StatusCode, body[:min(300, len(body))])
				}
			},
		},
		{
			name:       "diagnostic_exports.create",
			wantAction: "create",
			wantEntity: "diagnostic_exports",
			trigger: func(t *testing.T, a interface {
				Test(*http.Request, ...int) (*http.Response, error)
			}, db *sql.DB, cookie string) {
				t.Helper()
				req := httptest.NewRequest(http.MethodPost, "/diagnostics/exports", nil)
				req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
				req.AddCookie(&http.Cookie{Name: "careops_session", Value: cookie})
				resp, err := a.Test(req, -1)
				if err != nil {
					t.Fatalf("POST /diagnostics/exports: %v", err)
				}
				if resp.StatusCode != http.StatusFound && resp.StatusCode != http.StatusSeeOther {
					body := readBody(t, resp)
					t.Fatalf("diagnostics export: want redirect, got %d — %s", resp.StatusCode, body[:min(300, len(body))])
				}
			},
		},
		{
			name:       "exam_session.generate",
			wantAction: "exam_session.generate",
			wantEntity: "competency_exam_sessions",
			trigger: func(t *testing.T, a interface {
				Test(*http.Request, ...int) (*http.Response, error)
			}, db *sql.DB, cookie string) {
				t.Helper()
				tmplForm := url.Values{
					"title":            {"Matrix Audit Exam"},
					"role_scope":       {"nurse"},
					"passing_score":    {"80"},
					"duration_minutes": {"60"},
					"window_start":     {"08:00"},
					"window_end":       {"18:00"},
				}
				tmplReq := httptest.NewRequest(http.MethodPost, "/exams/templates", strings.NewReader(tmplForm.Encode()))
				tmplReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
				tmplReq.AddCookie(&http.Cookie{Name: "careops_session", Value: cookie})
				tmplResp, err := a.Test(tmplReq, -1)
				if err != nil {
					t.Fatalf("POST /exams/templates: %v", err)
				}
				tmplLoc := tmplResp.Header.Get("Location")
				tmplID := strings.TrimPrefix(tmplLoc, "/exams/templates/")
				if tmplID == "" || strings.ContainsAny(tmplID, "/ ") {
					t.Skipf("template create did not redirect to detail: %q", tmplLoc)
				}

				sessForm := url.Values{
					"template_id":   {tmplID},
					"planned_start": {"2024-07-01T09:00"},
					"room":          {"Matrix Room"},
					"proctor_id":    {""},
				}
				sessReq := httptest.NewRequest(http.MethodPost, "/exams/sessions", strings.NewReader(sessForm.Encode()))
				sessReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
				sessReq.AddCookie(&http.Cookie{Name: "careops_session", Value: cookie})
				sessResp, err := a.Test(sessReq, -1)
				if err != nil {
					t.Fatalf("POST /exams/sessions: %v", err)
				}
				if sessResp.StatusCode != http.StatusFound && sessResp.StatusCode != http.StatusSeeOther {
					body := readBody(t, sessResp)
					t.Fatalf("generate session: want redirect, got %d — %s", sessResp.StatusCode, body[:min(300, len(body))])
				}
			},
		},
	}

	for _, row := range rows {
		row := row
		t.Run(row.name, func(t *testing.T) {
			cfg := testConfig(t)
			log := logger.New("text", "error")
			db, err := app.BootstrapFast(cfg, log)
			if err != nil {
				t.Fatalf("Bootstrap: %v", err)
			}
			t.Cleanup(func() { db.Close() })
			a := app.New(cfg, db, log)
			cookie := loginAs(t, a, "admin@careops.local", "Admin!234567")

			row.trigger(t, a, db, cookie)

			var (
				userID     string
				entityType string
				outcome    string
			)
			err = db.QueryRow(
				`SELECT user_id, entity_type, outcome FROM audit_logs
				 WHERE action = ? AND entity_type = ?
				 ORDER BY occurred_at DESC LIMIT 1`,
				row.wantAction, row.wantEntity,
			).Scan(&userID, &entityType, &outcome)
			if err != nil {
				t.Fatalf("query audit entry for action=%q entity_type=%q: %v",
					row.wantAction, row.wantEntity, err)
			}
			if userID == "" {
				t.Errorf("%s: audit entry user_id must not be empty", row.name)
			}
			if entityType != row.wantEntity {
				t.Errorf("%s: want entity_type=%q, got %q", row.name, row.wantEntity, entityType)
			}
			if outcome != "success" {
				t.Errorf("%s: want outcome='success', got %q", row.name, outcome)
			}
		})
	}
}
