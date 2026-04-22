package idgen

import "github.com/google/uuid"

// New returns a new random UUIDv4 string.
func New() string {
	return uuid.NewString()
}
