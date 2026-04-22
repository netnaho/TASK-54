package timeutil

import "time"

// Clock is an interface that wraps time.Now() to allow deterministic testing.
type Clock interface {
	Now() time.Time
}

// RealClock returns actual wall-clock time.
type RealClock struct{}

func (RealClock) Now() time.Time { return time.Now().UTC() }

// NowISO returns the current UTC time formatted as an ISO-8601 string for storage in SQLite.
func NowISO() string {
	return time.Now().UTC().Format(time.RFC3339)
}
