package service

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"

	authdomain "careops/clinic/internal/modules/auth/domain"
	"careops/clinic/internal/modules/auth/repository"
	userdomain "careops/clinic/internal/modules/users/domain"
	userrepo "careops/clinic/internal/modules/users/repository"
	"careops/clinic/internal/shared/passwords"
)

var (
	ErrInvalidCredentials = errors.New("invalid email or password")
	ErrAccountDisabled    = errors.New("account is disabled")
	ErrSessionExpired     = errors.New("session has expired")
)

// AuthService handles login, logout, session resolution, and inactivity enforcement.
type AuthService struct {
	sessions                *repository.SessionRepo
	users                   *userrepo.UserRepo
	ttlHours                int
	inactivityTimeoutMinutes int
	log                     *slog.Logger
}

func NewAuthService(
	sessions *repository.SessionRepo,
	users *userrepo.UserRepo,
	ttlHours int,
	inactivityTimeoutMinutes int,
	log *slog.Logger,
) *AuthService {
	return &AuthService{
		sessions:                sessions,
		users:                   users,
		ttlHours:                ttlHours,
		inactivityTimeoutMinutes: inactivityTimeoutMinutes,
		log:                     log,
	}
}

// Login verifies credentials and creates a session.
// Returns the opaque session token, the authenticated user ID, and any error.
func (s *AuthService) Login(email, plainPassword, userAgent, ipAddress string) (string, string, error) {
	user, err := s.users.FindByEmail(email)
	if err != nil {
		if errors.Is(err, userrepo.ErrUserNotFound) {
			return "", "", ErrInvalidCredentials
		}
		return "", "", fmt.Errorf("auth: login: lookup user: %w", err)
	}

	if !user.IsActive {
		return "", "", ErrAccountDisabled
	}

	if err := passwords.Verify(plainPassword, user.PasswordHash); err != nil {
		return "", "", ErrInvalidCredentials
	}

	token, err := generateToken()
	if err != nil {
		return "", "", fmt.Errorf("auth: login: generate token: %w", err)
	}

	if _, err := s.sessions.Create(user.ID, token, userAgent, ipAddress, s.ttlHours); err != nil {
		return "", "", fmt.Errorf("auth: login: create session: %w", err)
	}

	s.log.Info("user logged in", "user_id", user.ID, "email", user.Email, "ip", ipAddress)
	return token, user.ID, nil
}

// Logout deletes the session identified by the raw token.
func (s *AuthService) Logout(rawToken string) error {
	return s.sessions.DeleteByToken(rawToken)
}

// ResolveSession returns the authenticated user for the given raw cookie token,
// enforcing both the absolute TTL and the 15-minute inactivity timeout.
func (s *AuthService) ResolveSession(rawToken string) (*userdomain.User, error) {
	sess, err := s.sessions.FindByToken(rawToken)
	if err != nil {
		if errors.Is(err, repository.ErrSessionNotFound) {
			return nil, ErrInvalidCredentials
		}
		return nil, err
	}

	if sess.IsExpired() {
		_ = s.sessions.DeleteByToken(rawToken)
		return nil, ErrSessionExpired
	}

	if sess.IsInactive(s.inactivityTimeoutMinutes) {
		_ = s.sessions.DeleteByToken(rawToken)
		return nil, ErrSessionExpired
	}

	user, err := s.users.FindByID(sess.UserID)
	if err != nil {
		return nil, fmt.Errorf("auth: resolve session: %w", err)
	}

	return user, nil
}

// ResolveSessionWithRoles returns the user with their role names loaded.
func (s *AuthService) ResolveSessionWithRoles(rawToken string) (*userdomain.User, error) {
	user, err := s.ResolveSession(rawToken)
	if err != nil {
		return nil, err
	}

	roles, err := s.users.FindRoles(user.ID)
	if err != nil {
		return nil, fmt.Errorf("auth: resolve roles: %w", err)
	}
	user.Roles = roles
	return user, nil
}

// TouchActivity updates the last-activity timestamp for the session.
// Called by RequireSession middleware after successful resolution.
func (s *AuthService) TouchActivity(rawToken string) error {
	return s.sessions.TouchActivity(rawToken)
}

// FindSession returns the raw session domain object (for audit logging context).
func (s *AuthService) FindSession(rawToken string) (*authdomain.Session, error) {
	return s.sessions.FindByToken(rawToken)
}

// — private —

func generateToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
