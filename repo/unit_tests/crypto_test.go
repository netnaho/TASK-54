package unit_tests

import (
	"strings"
	"testing"

	"careops/clinic/internal/shared/crypto"
)

// testKey is a 32-byte key used for unit tests only (must not be used in production).
var testKey = []byte("careops-test-key-32bytes-padding")

func TestCipher_EncryptDecryptRoundTrip(t *testing.T) {
	c, err := crypto.New(testKey)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	cases := []string{
		"CARD-5678-9012-3456",
		"INS-REF-00012345",
		"",
		"x",
		strings.Repeat("A", 1000),
	}

	for _, plain := range cases {
		enc, err := c.Encrypt(plain)
		if err != nil {
			t.Errorf("Encrypt(%q): %v", plain, err)
			continue
		}
		if enc == plain && plain != "" {
			t.Errorf("Encrypt returned plaintext unchanged for %q", plain)
		}

		dec, err := c.Decrypt(enc)
		if err != nil {
			t.Errorf("Decrypt(%q): %v", plain, err)
			continue
		}
		if dec != plain {
			t.Errorf("round-trip mismatch: want %q, got %q", plain, dec)
		}
	}
}

func TestCipher_DifferentCiphertextsForSamePlaintext(t *testing.T) {
	c, _ := crypto.New(testKey)

	enc1, _ := c.Encrypt("same-value")
	enc2, _ := c.Encrypt("same-value")

	if enc1 == enc2 {
		t.Error("expected different ciphertexts for same plaintext (random nonce)")
	}
}

func TestCipher_DecryptTamperedDataFails(t *testing.T) {
	c, _ := crypto.New(testKey)

	enc, err2 := c.Encrypt("secret")
	if err2 != nil {
		t.Fatalf("Encrypt for tamper test: %v", err2)
	}
	if len(enc) < 8 {
		t.Fatalf("ciphertext too short to tamper: %q", enc)
	}
	// Replace last 4 base64 chars to corrupt the authentication tag.
	tampered := enc[:len(enc)-4] + "XXXX"

	_, err := c.Decrypt(tampered)
	if err == nil {
		t.Error("expected error decrypting tampered ciphertext")
	}
}

func TestCipher_WrongKey(t *testing.T) {
	c1, _ := crypto.New(testKey)
	c2, _ := crypto.New([]byte("differentkey-32bytes-paddingXXXX"))

	enc, _ := c1.Encrypt("secret")
	_, err := c2.Decrypt(enc)
	if err == nil {
		t.Error("expected error decrypting with wrong key")
	}
}

func TestCipher_InvalidKeyLength(t *testing.T) {
	_, err := crypto.New([]byte("tooshort"))
	if err == nil {
		t.Error("expected error for key shorter than 32 bytes")
	}
}

func TestCipher_NotReady(t *testing.T) {
	var c *crypto.Cipher
	if c.IsReady() {
		t.Error("nil cipher should not be ready")
	}
	_, err := c.Encrypt("test")
	if err == nil {
		t.Error("expected error encrypting with nil cipher")
	}
	_, err = c.Decrypt("test")
	if err == nil {
		t.Error("expected error decrypting with nil cipher")
	}
}

func TestMaskString(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"CARD-5678-9012", "****9012"},
		{"1234", "****"},
		{"AB12", "****"},
		{"ABCDE", "****BCDE"},
		{"", ""},
		{"INS-REF-00012345", "****2345"},
	}
	for _, tc := range cases {
		got := crypto.MaskString(tc.input)
		if got != tc.want {
			t.Errorf("MaskString(%q): want %q, got %q", tc.input, tc.want, got)
		}
	}
}

func TestRedactForLog(t *testing.T) {
	if got := crypto.RedactForLog("sensitive"); got != "[REDACTED]" {
		t.Errorf("RedactForLog: want [REDACTED], got %q", got)
	}
	if got := crypto.RedactForLog(""); got != "[REDACTED]" {
		t.Errorf("RedactForLog empty: want [REDACTED], got %q", got)
	}
}
