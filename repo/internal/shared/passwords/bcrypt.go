package passwords

import (
	"fmt"

	"golang.org/x/crypto/bcrypt"
)

const DefaultCost = 10

// Hash returns a bcrypt hash of the plaintext password.
func Hash(plaintext string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(plaintext), DefaultCost)
	if err != nil {
		return "", fmt.Errorf("passwords: hash: %w", err)
	}
	return string(b), nil
}

// Verify returns nil when plaintext matches the stored hash, or an error otherwise.
// The error is bcrypt.ErrMismatchedHashAndPassword for wrong passwords.
func Verify(plaintext, hash string) error {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(plaintext))
}
