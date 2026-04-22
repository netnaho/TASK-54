package unit_tests

import (
	"testing"
	"time"

	"careops/clinic/internal/modules/auth/domain"
)

// ── Session.IsExpired ─────────────────────────────────────────────────────────

func TestSession_IsExpired_FutureTTL_ReturnsFalse(t *testing.T) {
	s := domain.Session{
		ExpiresAt: time.Now().UTC().Add(1 * time.Hour),
	}
	if s.IsExpired() {
		t.Error("IsExpired: expected false when ExpiresAt is in the future")
	}
}

func TestSession_IsExpired_PastTTL_ReturnsTrue(t *testing.T) {
	s := domain.Session{
		ExpiresAt: time.Now().UTC().Add(-1 * time.Second),
	}
	if !s.IsExpired() {
		t.Error("IsExpired: expected true when ExpiresAt is in the past")
	}
}

func TestSession_IsExpired_ExactlyNow_IsExpired(t *testing.T) {
	// A session expiring exactly "now" (or just past) is considered expired.
	s := domain.Session{
		ExpiresAt: time.Now().UTC().Add(-1 * time.Millisecond),
	}
	if !s.IsExpired() {
		t.Error("IsExpired: expected true for a session expired by 1ms")
	}
}

// ── Session.IsInactive ────────────────────────────────────────────────────────

func TestSession_IsInactive_RecentActivity_ReturnsFalse(t *testing.T) {
	s := domain.Session{
		LastActivityAt: time.Now().UTC().Add(-5 * time.Minute),
		CreatedAt:      time.Now().UTC().Add(-10 * time.Minute),
		ExpiresAt:      time.Now().UTC().Add(24 * time.Hour),
	}
	// 15-minute timeout, last activity 5 minutes ago → active
	if s.IsInactive(15) {
		t.Error("IsInactive: expected false when last activity was 5 min ago with 15 min timeout")
	}
}

func TestSession_IsInactive_StaleActivity_ReturnsTrue(t *testing.T) {
	s := domain.Session{
		LastActivityAt: time.Now().UTC().Add(-20 * time.Minute),
		CreatedAt:      time.Now().UTC().Add(-25 * time.Minute),
		ExpiresAt:      time.Now().UTC().Add(24 * time.Hour),
	}
	// 15-minute timeout, last activity 20 minutes ago → inactive
	if !s.IsInactive(15) {
		t.Error("IsInactive: expected true when last activity was 20 min ago with 15 min timeout")
	}
}

func TestSession_IsInactive_ZeroLastActivity_FallsBackToCreatedAt(t *testing.T) {
	// LastActivityAt zero → fall back to CreatedAt (backward-compat with pre-migration rows)
	s := domain.Session{
		LastActivityAt: time.Time{}, // zero
		CreatedAt:      time.Now().UTC().Add(-20 * time.Minute),
		ExpiresAt:      time.Now().UTC().Add(24 * time.Hour),
	}
	if !s.IsInactive(15) {
		t.Error("IsInactive: expected true; zero LastActivityAt should fall back to CreatedAt (20 min ago)")
	}
}

func TestSession_IsInactive_ZeroLastActivity_ActiveWhenCreatedAtRecent(t *testing.T) {
	s := domain.Session{
		LastActivityAt: time.Time{}, // zero
		CreatedAt:      time.Now().UTC().Add(-5 * time.Minute),
		ExpiresAt:      time.Now().UTC().Add(24 * time.Hour),
	}
	if s.IsInactive(15) {
		t.Error("IsInactive: expected false; CreatedAt is 5 min ago within 15 min timeout")
	}
}

func TestSession_IsInactive_ExactlyAtTimeout_IsInactive(t *testing.T) {
	// Session last used exactly at timeout boundary is considered inactive.
	s := domain.Session{
		LastActivityAt: time.Now().UTC().Add(-15*time.Minute - 1*time.Millisecond),
		CreatedAt:      time.Now().UTC().Add(-20 * time.Minute),
		ExpiresAt:      time.Now().UTC().Add(24 * time.Hour),
	}
	if !s.IsInactive(15) {
		t.Error("IsInactive: expected true at exactly past timeout threshold")
	}
}

func TestSession_IsInactive_OneMinuteTimeout_Strict(t *testing.T) {
	s := domain.Session{
		LastActivityAt: time.Now().UTC().Add(-90 * time.Second),
		CreatedAt:      time.Now().UTC().Add(-2 * time.Minute),
		ExpiresAt:      time.Now().UTC().Add(24 * time.Hour),
	}
	// 1-minute timeout, last activity 90 seconds ago → inactive
	if !s.IsInactive(1) {
		t.Error("IsInactive: expected true for 90s inactive with 1-minute timeout")
	}
}
