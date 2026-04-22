// Package crypto provides AES-256-GCM field encryption and masking helpers
// for protecting sensitive values such as payer reference numbers.
//
// # Encryption strategy
//
// Encrypted values are stored as base64(nonce || ciphertext) in TEXT columns.
// The 12-byte GCM nonce is randomly generated per encryption so identical
// plaintexts produce different ciphertexts, preventing frequency analysis.
//
// # Key loading
//
// The key is loaded from:
//   1. The FIELD_ENCRYPT_KEY environment variable — expected as a 64-character
//      hex string (32 bytes).
//   2. The file path given in FIELD_ENCRYPT_KEY_FILE — same hex format.
//
// No key configured → the Cipher is nil. Callers must guard with IsReady()
// before encrypting writes. In that state, reads return the raw stored value
// and masking still applies (the mask operates on the stored string, not the
// decrypted one).
//
// # Masking strategy
//
// Non-finance views call MaskString on any reference/card/payer field before
// rendering. Finance views decrypt and display the full value.
// Logs always call RedactForLog — raw reference numbers never appear in logs.
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
)

// Cipher wraps AES-256-GCM operations. A nil *Cipher means encryption is
// not configured; callers must check IsReady() before encrypting new data.
type Cipher struct {
	key []byte // exactly 32 bytes
}

// LoadFromEnv constructs a Cipher by reading FIELD_ENCRYPT_KEY (hex) or the
// file at FIELD_ENCRYPT_KEY_FILE. Returns (nil, nil) if neither is set —
// callers must handle the unconfigured case gracefully.
func LoadFromEnv() (*Cipher, error) {
	if raw := os.Getenv("FIELD_ENCRYPT_KEY"); raw != "" {
		return fromHex(raw)
	}
	if path := os.Getenv("FIELD_ENCRYPT_KEY_FILE"); path != "" {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("crypto: read key file %s: %w", path, err)
		}
		return fromHex(strings.TrimSpace(string(data)))
	}
	return nil, nil
}

// New creates a Cipher directly from a 32-byte key slice (useful in tests).
func New(key []byte) (*Cipher, error) {
	if len(key) != 32 {
		return nil, fmt.Errorf("crypto: key must be 32 bytes, got %d", len(key))
	}
	k := make([]byte, 32)
	copy(k, key)
	return &Cipher{key: k}, nil
}

// IsReady reports whether encryption is configured.
func (c *Cipher) IsReady() bool { return c != nil && len(c.key) == 32 }

// Encrypt returns base64(nonce || GCM-sealed ciphertext) for the given plaintext.
// Returns an error if the cipher is not ready.
func (c *Cipher) Encrypt(plaintext string) (string, error) {
	if !c.IsReady() {
		return "", errors.New("crypto: encryption key not configured")
	}
	block, err := aes.NewCipher(c.key)
	if err != nil {
		return "", fmt.Errorf("crypto: new cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("crypto: new GCM: %w", err)
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("crypto: generate nonce: %w", err)
	}
	sealed := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(sealed), nil
}

// Decrypt reverses Encrypt. The input must be the base64 string returned by Encrypt.
func (c *Cipher) Decrypt(encoded string) (string, error) {
	if !c.IsReady() {
		return "", errors.New("crypto: encryption key not configured")
	}
	sealed, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", fmt.Errorf("crypto: base64 decode: %w", err)
	}
	block, err := aes.NewCipher(c.key)
	if err != nil {
		return "", fmt.Errorf("crypto: new cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("crypto: new GCM: %w", err)
	}
	if len(sealed) < gcm.NonceSize() {
		return "", errors.New("crypto: ciphertext too short")
	}
	nonce, ct := sealed[:gcm.NonceSize()], sealed[gcm.NonceSize():]
	plain, err := gcm.Open(nil, nonce, ct, nil)
	if err != nil {
		return "", fmt.Errorf("crypto: decrypt: authentication failed")
	}
	return string(plain), nil
}

// MaskString redacts all but the last 4 characters of s for non-finance views.
// Examples: "CARD-5678-9012" → "****9012"; "AB12" → "****"; "" → "".
// Apply this to any sensitive reference/card/payer field before rendering in
// templates that are accessible to non-finance roles.
func MaskString(s string) string {
	if s == "" {
		return ""
	}
	const visible = 4
	if len(s) <= visible {
		return "****"
	}
	return "****" + s[len(s)-visible:]
}

// RedactForLog replaces the value with "[REDACTED]" for structured log fields.
// Use this whenever logging any payment, card, or reference number field.
func RedactForLog(_ string) string { return "[REDACTED]" }

// — internal —

func fromHex(hexStr string) (*Cipher, error) {
	key, err := hex.DecodeString(hexStr)
	if err != nil {
		return nil, fmt.Errorf("crypto: decode hex key: %w", err)
	}
	return New(key)
}
