package unit_tests

import (
	"testing"

	"careops/clinic/internal/shared/validation"
)

func TestPasswordPolicy_Valid(t *testing.T) {
	valid := []string{
		"Admin!2345##",
		"Tr0ub4dor&33!",
		"Correct-Horse-1!",
		"MyS3cur3P@ssword",
	}
	for _, pw := range valid {
		if err := validation.PasswordPolicy(pw); err != nil {
			t.Errorf("PasswordPolicy(%q) should be valid, got: %v", pw, err)
		}
	}
}

func TestPasswordPolicy_TooShort(t *testing.T) {
	short := []string{"Ab1!", "Short1!", "11CharPas!"}
	for _, pw := range short {
		if err := validation.PasswordPolicy(pw); err == nil {
			t.Errorf("PasswordPolicy(%q) should fail (too short)", pw)
		}
	}
}

func TestPasswordPolicy_MissingRequirements(t *testing.T) {
	cases := []struct {
		pw   string
		desc string
	}{
		{"alllowercase1!", "no uppercase"},
		{"ALLUPPERCASE1!", "no lowercase"},
		{"NoDigitsAtAll!!", "no digit"},
		{"NoSpecialChars12", "no special char"},
	}
	for _, tc := range cases {
		if err := validation.PasswordPolicy(tc.pw); err == nil {
			t.Errorf("PasswordPolicy(%q) should fail (%s)", tc.pw, tc.desc)
		}
	}
}

func TestEmail_Valid(t *testing.T) {
	valid := []string{
		"admin@careops.local",
		"user+tag@example.com",
		"a@b.co",
	}
	for _, e := range valid {
		if err := validation.Email(e); err != nil {
			t.Errorf("Email(%q) should be valid: %v", e, err)
		}
	}
}

func TestEmail_Invalid(t *testing.T) {
	invalid := []string{"", "notanemail", "@nodomain", "noat.com", "missing@"}
	for _, e := range invalid {
		if err := validation.Email(e); err == nil {
			t.Errorf("Email(%q) should be invalid", e)
		}
	}
}

func TestLoginInputs_NormalisesEmail(t *testing.T) {
	email, err := validation.LoginInputs("  ADMIN@CareOps.LOCAL  ", "somepassword")
	if err != nil {
		t.Fatalf("LoginInputs: %v", err)
	}
	if email != "admin@careops.local" {
		t.Errorf("expected lowercased trimmed email, got %q", email)
	}
}

func TestLoginInputs_EmptyPasswordFails(t *testing.T) {
	_, err := validation.LoginInputs("user@example.com", "")
	if err == nil {
		t.Error("LoginInputs with empty password should fail")
	}
}

func TestIdempotencyKey_Valid(t *testing.T) {
	if err := validation.IdempotencyKey("abc-123"); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestIdempotencyKey_TooLong(t *testing.T) {
	long := make([]byte, 129)
	for i := range long {
		long[i] = 'a'
	}
	if err := validation.IdempotencyKey(string(long)); err == nil {
		t.Error("expected error for key > 128 chars")
	}
}

func TestPathSegment_Traversal(t *testing.T) {
	bad := []string{"../etc/passwd", "foo/bar", "back\\slash", "../../secret"}
	for _, s := range bad {
		if err := validation.PathSegment("file", s); err == nil {
			t.Errorf("PathSegment(%q) should fail", s)
		}
	}
}

func TestPathSegment_Valid(t *testing.T) {
	good := []string{"report-2024-01.csv", "snapshot_v3", "export123"}
	for _, s := range good {
		if err := validation.PathSegment("file", s); err != nil {
			t.Errorf("PathSegment(%q) should pass: %v", s, err)
		}
	}
}

func TestRowVersion_Valid(t *testing.T) {
	if err := validation.RowVersion(1); err != nil {
		t.Errorf("RowVersion(1): %v", err)
	}
	if err := validation.RowVersion(999); err != nil {
		t.Errorf("RowVersion(999): %v", err)
	}
}

func TestRowVersion_Invalid(t *testing.T) {
	for _, v := range []int{0, -1, -100} {
		if err := validation.RowVersion(v); err == nil {
			t.Errorf("RowVersion(%d) should fail", v)
		}
	}
}
