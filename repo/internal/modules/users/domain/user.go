package domain

import "time"

// User represents a staff member with login access to the system.
type User struct {
	ID           string
	Email        string
	DisplayName  string
	PasswordHash string
	IsActive     bool
	RowVersion   int
	CreatedAt    time.Time
	UpdatedAt    time.Time

	// Roles is loaded separately and populated by the user repository on demand.
	Roles []string
}
