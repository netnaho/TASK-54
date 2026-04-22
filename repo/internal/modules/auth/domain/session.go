package domain

import "time"

// Session represents an authenticated browser session stored in the database.
type Session struct {
	ID             string
	UserID         string
	TokenHash      string    // SHA-256 hex of the opaque cookie value
	UserAgent      string
	IPAddress      string
	ExpiresAt      time.Time
	LastActivityAt time.Time // updated on every authenticated request; zero ⟹ use CreatedAt
	CreatedAt      time.Time
}

// IsExpired reports whether the absolute TTL has passed.
func (s Session) IsExpired() bool {
	return time.Now().UTC().After(s.ExpiresAt)
}

// IsInactive reports whether the session has been idle for longer than timeoutMinutes.
// A zero LastActivityAt falls back to CreatedAt for backward-compat with pre-0003 rows.
func (s Session) IsInactive(timeoutMinutes int) bool {
	base := s.LastActivityAt
	if base.IsZero() {
		base = s.CreatedAt
	}
	deadline := base.Add(time.Duration(timeoutMinutes) * time.Minute)
	return time.Now().UTC().After(deadline)
}
