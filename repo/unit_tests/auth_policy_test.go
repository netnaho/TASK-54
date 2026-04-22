package unit_tests

import (
	"testing"

	"careops/clinic/internal/modules/auth/policy"
)

// ── HasRole ──────────────────────────────────────────────────────────────────

func TestHasRole_ReturnsTrue_WhenRolePresent(t *testing.T) {
	roles := []string{"admin", "nurse"}
	if !policy.HasRole(roles, "admin") {
		t.Error("HasRole: expected true for 'admin' in ['admin','nurse']")
	}
}

func TestHasRole_ReturnsFalse_WhenRoleAbsent(t *testing.T) {
	roles := []string{"nurse", "therapist"}
	if policy.HasRole(roles, "admin") {
		t.Error("HasRole: expected false for 'admin' not in ['nurse','therapist']")
	}
}

func TestHasRole_EmptyRoles_ReturnsFalse(t *testing.T) {
	if policy.HasRole([]string{}, "admin") {
		t.Error("HasRole: expected false for empty role list")
	}
}

func TestHasRole_NilRoles_ReturnsFalse(t *testing.T) {
	if policy.HasRole(nil, "admin") {
		t.Error("HasRole: expected false for nil role list")
	}
}

func TestHasRole_ExactMatch_CaseSensitive(t *testing.T) {
	roles := []string{"Admin", "NURSE"}
	if policy.HasRole(roles, "admin") {
		t.Error("HasRole: must be case-sensitive; 'Admin' should not match 'admin'")
	}
}

// ── HasAnyRole ────────────────────────────────────────────────────────────────

func TestHasAnyRole_ReturnsTrue_WhenOneMatches(t *testing.T) {
	roles := []string{"nurse", "aide"}
	if !policy.HasAnyRole(roles, "admin", "nurse") {
		t.Error("HasAnyRole: expected true; 'nurse' is in targets")
	}
}

func TestHasAnyRole_ReturnsTrue_WhenAllMatch(t *testing.T) {
	roles := []string{"admin", "auditor"}
	if !policy.HasAnyRole(roles, "admin", "auditor") {
		t.Error("HasAnyRole: expected true; both roles present")
	}
}

func TestHasAnyRole_ReturnsFalse_WhenNoneMatch(t *testing.T) {
	roles := []string{"aide", "front_desk"}
	if policy.HasAnyRole(roles, "admin", "finance_clerk") {
		t.Error("HasAnyRole: expected false; neither 'admin' nor 'finance_clerk' present")
	}
}

func TestHasAnyRole_EmptyTargets_ReturnsFalse(t *testing.T) {
	roles := []string{"admin", "auditor"}
	if policy.HasAnyRole(roles) {
		t.Error("HasAnyRole: expected false for empty targets variadic")
	}
}

func TestHasAnyRole_EmptyRoles_ReturnsFalse(t *testing.T) {
	if policy.HasAnyRole([]string{}, "admin", "nurse") {
		t.Error("HasAnyRole: expected false for empty role list")
	}
}

// ── IsAdmin ───────────────────────────────────────────────────────────────────

func TestIsAdmin_ReturnsTrue_ForAdmin(t *testing.T) {
	if !policy.IsAdmin([]string{"admin", "nurse"}) {
		t.Error("IsAdmin: expected true when 'admin' is present")
	}
}

func TestIsAdmin_ReturnsFalse_ForNonAdmin(t *testing.T) {
	if policy.IsAdmin([]string{"nurse", "auditor", "finance_clerk"}) {
		t.Error("IsAdmin: expected false when 'admin' is absent")
	}
}

func TestIsAdmin_ReturnsFalse_EmptyRoles(t *testing.T) {
	if policy.IsAdmin(nil) {
		t.Error("IsAdmin: expected false for nil roles")
	}
}

// ── Role constant values ──────────────────────────────────────────────────────

func TestRoleConstants_ValuesMatchSeeds(t *testing.T) {
	// These must stay in sync with seeds/0001_roles.sql
	expected := map[string]string{
		"admin":                policy.RoleAdmin,
		"auditor":              policy.RoleAuditor,
		"nurse":                policy.RoleNurse,
		"front_desk":           policy.RoleFrontDesk,
		"therapist":            policy.RoleTherapist,
		"aide":                 policy.RoleAide,
		"training_coordinator": policy.RoleTrainingCoordinator,
		"finance_clerk":        policy.RoleFinanceClerk,
	}
	for want, got := range expected {
		if want != got {
			t.Errorf("role constant mismatch: want %q, got %q", want, got)
		}
	}
}
