// Package validation provides reusable input validation helpers.
// All functions return a plain error with a human-readable message
// suitable for direct display in form error fields.
package validation

import (
	"fmt"
	"strings"
	"unicode"
)

// PasswordPolicy enforces the minimum security requirements for new or reset passwords.
// Rules: ≥12 characters; ≥1 uppercase, ≥1 lowercase, ≥1 digit, ≥1 special character.
func PasswordPolicy(password string) error {
	if len(password) < 12 {
		return fmt.Errorf("password must be at least 12 characters")
	}
	var hasUpper, hasLower, hasDigit, hasSpecial bool
	for _, r := range password {
		switch {
		case unicode.IsUpper(r):
			hasUpper = true
		case unicode.IsLower(r):
			hasLower = true
		case unicode.IsDigit(r):
			hasDigit = true
		case unicode.IsPunct(r) || unicode.IsSymbol(r):
			hasSpecial = true
		}
	}
	var missing []string
	if !hasUpper {
		missing = append(missing, "uppercase letter")
	}
	if !hasLower {
		missing = append(missing, "lowercase letter")
	}
	if !hasDigit {
		missing = append(missing, "digit")
	}
	if !hasSpecial {
		missing = append(missing, "special character")
	}
	if len(missing) > 0 {
		return fmt.Errorf("password must contain at least one %s", strings.Join(missing, ", "))
	}
	return nil
}

// Email returns an error if s does not have the shape of a valid address.
// This is a structural check only; it does not verify deliverability.
func Email(s string) error {
	s = strings.TrimSpace(s)
	if s == "" {
		return fmt.Errorf("email is required")
	}
	at := strings.LastIndex(s, "@")
	if at <= 0 || at == len(s)-1 {
		return fmt.Errorf("invalid email address")
	}
	domain := s[at+1:]
	if !strings.Contains(domain, ".") || strings.HasSuffix(domain, ".") {
		return fmt.Errorf("invalid email address")
	}
	return nil
}

// Required returns an error if the trimmed value is empty.
func Required(field, value string) error {
	if strings.TrimSpace(value) == "" {
		return fmt.Errorf("%s is required", field)
	}
	return nil
}

// MaxLength returns an error if s exceeds n runes.
func MaxLength(field, s string, n int) error {
	if len([]rune(s)) > n {
		return fmt.Errorf("%s must not exceed %d characters", field, n)
	}
	return nil
}

// IdempotencyKey validates an X-Idempotency-Key header value.
// Max 128 characters; must not be empty.
func IdempotencyKey(key string) error {
	if strings.TrimSpace(key) == "" {
		return fmt.Errorf("idempotency key is required")
	}
	return MaxLength("idempotency key", key, 128)
}

// PathSegment returns an error if s contains path traversal characters.
// Use this before incorporating any user-supplied string into a file path.
func PathSegment(field, s string) error {
	if strings.Contains(s, "..") || strings.ContainsAny(s, "/\\") {
		return fmt.Errorf("%s contains invalid path characters", field)
	}
	return nil
}

// RowVersion returns an error if v would be an invalid optimistic lock version (≤0).
func RowVersion(v int) error {
	if v <= 0 {
		return fmt.Errorf("row_version must be a positive integer, got %d", v)
	}
	return nil
}

// LoginInputs validates the email and password fields from the login form.
// Returns (email, err) where email is trimmed and lowercased.
func LoginInputs(email, password string) (string, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if err := Email(email); err != nil {
		return "", err
	}
	if err := Required("password", password); err != nil {
		return "", err
	}
	return email, nil
}
