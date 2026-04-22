// Package idempotency provides a reusable idempotency key service backed by SQLite.
//
// # Strategy
//
// Callers supply an X-Idempotency-Key request header (or _idempotency_key form field).
// On the first call the service stores the key with the response after the handler
// completes. On duplicate calls within the 24-hour window the stored response is
// returned directly, preventing double-submission of creates, approvals, or exports.
//
// Usage pattern for handlers:
//
//	key := idempotency.ExtractKey(c)
//	if key != "" {
//	    if entry, err := idemSvc.Check(key); err == nil && entry != nil {
//	        return c.Status(entry.ResponseStatus).SendString(entry.ResponseBody)
//	    }
//	}
//	// ... perform the operation ...
//	if key != "" {
//	    _ = idemSvc.Store(key, status, responseBody)
//	}
package idempotency

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"careops/clinic/internal/shared/timeutil"
	"github.com/gofiber/fiber/v2"
)

const ttl = 24 * time.Hour

// Entry is a previously stored idempotency record.
type Entry struct {
	Key            string
	ResponseStatus int
	ResponseBody   string
	CreatedAt      time.Time
	ExpiresAt      time.Time
}

// Service checks and persists idempotency keys.
type Service struct {
	db *sql.DB
}

// NewService creates a Service backed by the given database.
func NewService(db *sql.DB) *Service { return &Service{db: db} }

// Check looks up the key. Returns:
//   - (entry, nil)  — key found and not expired; caller should replay the stored response
//   - (nil, nil)    — key not found or expired; caller should proceed normally
//   - (nil, err)    — database error
func (s *Service) Check(key string) (*Entry, error) {
	var e Entry
	var createdAt, expiresAt string
	err := s.db.QueryRow(`
		SELECT key, response_status, response_body, created_at, expires_at
		FROM idempotency_keys WHERE key = ?`, key,
	).Scan(&e.Key, &e.ResponseStatus, &e.ResponseBody, &createdAt, &expiresAt)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("idempotency.Check: %w", err)
	}

	e.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	e.ExpiresAt, _ = time.Parse(time.RFC3339, expiresAt)

	if time.Now().UTC().After(e.ExpiresAt) {
		_, _ = s.db.Exec(`DELETE FROM idempotency_keys WHERE key = ?`, key)
		return nil, nil
	}
	return &e, nil
}

// Store persists a key with the handler's response for future replay.
// ON CONFLICT DO NOTHING prevents races on concurrent duplicate requests.
func (s *Service) Store(key string, status int, body string) error {
	now := timeutil.NowISO()
	expires := time.Now().UTC().Add(ttl).Format(time.RFC3339)
	_, err := s.db.Exec(`
		INSERT INTO idempotency_keys (key, response_status, response_body, created_at, expires_at)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(key) DO NOTHING`,
		key, status, body, now, expires,
	)
	return err
}

// Cleanup deletes all expired entries. Intended to be called on a schedule.
func (s *Service) Cleanup() (int64, error) {
	res, err := s.db.Exec(`DELETE FROM idempotency_keys WHERE expires_at < ?`,
		time.Now().UTC().Format(time.RFC3339))
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// ExtractKey reads the idempotency key from the request.
// Priority: X-Idempotency-Key header, then _idempotency_key form field.
// Returns "" if neither is present.
func ExtractKey(c *fiber.Ctx) string {
	if k := c.Get("X-Idempotency-Key"); k != "" {
		return k
	}
	return c.FormValue("_idempotency_key")
}
