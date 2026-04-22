package service

import (
	"fmt"
	"strings"

	"careops/clinic/internal/modules/users/domain"
	"careops/clinic/internal/modules/users/repository"
	"careops/clinic/internal/shared/passwords"
	"careops/clinic/internal/shared/validation"
)

// UserService provides business-layer operations on users.
type UserService struct {
	repo *repository.UserRepo
}

func NewUserService(repo *repository.UserRepo) *UserService {
	return &UserService{repo: repo}
}

// ListWithRoles returns all users with their role names populated.
func (s *UserService) ListWithRoles() ([]domain.User, error) {
	users, err := s.repo.List()
	if err != nil {
		return nil, err
	}
	for i := range users {
		roles, err := s.repo.FindRoles(users[i].ID)
		if err != nil {
			return nil, err
		}
		users[i].Roles = roles
	}
	return users, nil
}

func (s *UserService) Count() (int, error) { return s.repo.Count() }

func (s *UserService) ListRoles() ([]domain.Role, error) { return s.repo.ListRoles() }

func (s *UserService) FindByID(id string) (*domain.User, error) { return s.repo.FindByID(id) }

// CreateUser validates the command, enforces PasswordPolicy, hashes the
// password, and persists a new user with an optional initial role.
func (s *UserService) CreateUser(cmd domain.CreateUserCmd) (*domain.User, error) {
	cmd.Email = strings.ToLower(strings.TrimSpace(cmd.Email))
	if err := cmd.Validate(); err != nil {
		return nil, err
	}

	hash, err := passwords.Hash(cmd.Password)
	if err != nil {
		return nil, fmt.Errorf("users: hash password: %w", err)
	}

	// Resolve role name → ID if provided.
	roleID := ""
	if cmd.RoleID != "" {
		id, err := s.repo.FindRoleByName(cmd.RoleID)
		if err != nil {
			return nil, fmt.Errorf("users: role %q not found", cmd.RoleID)
		}
		roleID = id
	}

	u := &domain.User{
		Email:        cmd.Email,
		DisplayName:  cmd.DisplayName,
		PasswordHash: hash,
		IsActive:     true,
	}
	if err := s.repo.Create(u, roleID); err != nil {
		return nil, fmt.Errorf("users: create: %w", err)
	}
	return u, nil
}

// ResetPassword enforces PasswordPolicy, hashes the new password, and
// persists it with an optimistic concurrency check on cmd.RowVersion.
// Returns apperr.ErrConflict when the stored row_version no longer matches
// (concurrent modification since the form was loaded).
func (s *UserService) ResetPassword(cmd domain.ResetPasswordCmd) error {
	if err := validation.PasswordPolicy(cmd.Password); err != nil {
		return err
	}
	hash, err := passwords.Hash(cmd.Password)
	if err != nil {
		return fmt.Errorf("users: hash password: %w", err)
	}
	return s.repo.UpdatePasswordHashWithVersion(cmd.UserID, hash, cmd.RowVersion)
}
