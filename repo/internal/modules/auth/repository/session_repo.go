package repository

import (
	"crypto/sha256"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"careops/clinic/internal/modules/auth/domain"
	"careops/clinic/internal/shared/idgen"
)

var ErrSessionNotFound = errors.New("session not found")

type SessionRepo struct {
	db *sql.DB
}

func NewSessionRepo(db *sql.DB) *SessionRepo {
	return &SessionRepo{db: db}
}

// Create inserts a new session. The raw opaque token is hashed; it never touches the DB.
func (r *SessionRepo) Create(userID, rawToken, userAgent, ipAddress string, ttlHours int) (*domain.Session, error) {
	hash := hashToken(rawToken)
	now := time.Now().UTC()
	expiresAt := now.Add(time.Duration(ttlHours) * time.Hour)
	id := idgen.New()

	_, err := r.db.Exec(`
		INSERT INTO sessions
		  (id, user_id, token_hash, user_agent, ip_address, expires_at, last_activity_at, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		id, userID, hash, userAgent, ipAddress,
		expiresAt.Format(time.RFC3339),
		now.Format(time.RFC3339),
		now.Format(time.RFC3339),
	)
	if err != nil {
		return nil, fmt.Errorf("session_repo.Create: %w", err)
	}

	return &domain.Session{
		ID:             id,
		UserID:         userID,
		TokenHash:      hash,
		UserAgent:      userAgent,
		IPAddress:      ipAddress,
		ExpiresAt:      expiresAt,
		LastActivityAt: now,
		CreatedAt:      now,
	}, nil
}

// FindByToken looks up a session by the raw opaque token.
func (r *SessionRepo) FindByToken(rawToken string) (*domain.Session, error) {
	hash := hashToken(rawToken)
	row := r.db.QueryRow(`
		SELECT id, user_id, token_hash, user_agent, ip_address,
		       expires_at, last_activity_at, created_at
		FROM sessions WHERE token_hash = ?`, hash)

	var s domain.Session
	var expiresAt, lastActivity, createdAt string
	err := row.Scan(
		&s.ID, &s.UserID, &s.TokenHash, &s.UserAgent, &s.IPAddress,
		&expiresAt, &lastActivity, &createdAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrSessionNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("session_repo.FindByToken: %w", err)
	}

	s.ExpiresAt, _ = time.Parse(time.RFC3339, expiresAt)
	s.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	// lastActivity may be empty for rows created before migration 0003.
	if lastActivity != "" {
		s.LastActivityAt, _ = time.Parse(time.RFC3339, lastActivity)
	}
	return &s, nil
}

// TouchActivity updates last_activity_at for the session identified by the raw token.
// It is a no-op if the session is not found (token already deleted).
func (r *SessionRepo) TouchActivity(rawToken string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := r.db.Exec(
		`UPDATE sessions SET last_activity_at = ? WHERE token_hash = ?`,
		now, hashToken(rawToken),
	)
	return err
}

// DeleteByToken removes the session matching the raw token.
func (r *SessionRepo) DeleteByToken(rawToken string) error {
	_, err := r.db.Exec(`DELETE FROM sessions WHERE token_hash = ?`, hashToken(rawToken))
	return err
}

// DeleteExpired removes all sessions whose expires_at is in the past.
func (r *SessionRepo) DeleteExpired() (int64, error) {
	res, err := r.db.Exec(`DELETE FROM sessions WHERE expires_at < ?`,
		time.Now().UTC().Format(time.RFC3339))
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

func hashToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return fmt.Sprintf("%x", sum)
}
