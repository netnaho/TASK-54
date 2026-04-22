package unit_tests

import (
	"testing"

	"careops/clinic/internal/shared/passwords"
)

func TestPassword_HashAndVerify(t *testing.T) {
	plain := "TestPassword!9"

	hash, err := passwords.Hash(plain)
	if err != nil {
		t.Fatalf("Hash() error: %v", err)
	}
	if hash == "" {
		t.Fatal("Hash() returned empty string")
	}
	if hash == plain {
		t.Fatal("Hash() returned plaintext — that is wrong")
	}

	// Correct password must verify
	if err := passwords.Verify(plain, hash); err != nil {
		t.Errorf("Verify() failed for correct password: %v", err)
	}
}

func TestPassword_WrongPassword(t *testing.T) {
	hash, err := passwords.Hash("CorrectPassword!1")
	if err != nil {
		t.Fatalf("Hash() error: %v", err)
	}

	err = passwords.Verify("WrongPassword!2", hash)
	if err == nil {
		t.Error("Verify() returned nil for wrong password — expected error")
	}
}

func TestPassword_HashIsDeterministicallyDifferent(t *testing.T) {
	// bcrypt produces different hashes on every call (random salt)
	h1, _ := passwords.Hash("SamePassword!1")
	h2, _ := passwords.Hash("SamePassword!1")
	if h1 == h2 {
		t.Error("expected different bcrypt hashes for same input (random salt)")
	}

	// Both should still verify correctly
	if err := passwords.Verify("SamePassword!1", h1); err != nil {
		t.Errorf("Verify failed for h1: %v", err)
	}
	if err := passwords.Verify("SamePassword!1", h2); err != nil {
		t.Errorf("Verify failed for h2: %v", err)
	}
}
