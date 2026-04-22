package domain

import (
	"fmt"
	"strings"

	"careops/clinic/internal/shared/validation"
)

// CreateUserCmd carries validated input for creating a new staff user.
type CreateUserCmd struct {
	Email       string
	DisplayName string
	Password    string
	RoleID      string // optional; at most one role assigned at creation time
}

// Validate returns an error if any required field is missing or violates policy.
func (c *CreateUserCmd) Validate() error {
	if err := validation.Email(c.Email); err != nil {
		return err
	}
	if strings.TrimSpace(c.DisplayName) == "" {
		return fmt.Errorf("display name is required")
	}
	if err := validation.PasswordPolicy(c.Password); err != nil {
		return err
	}
	return nil
}

// ResetPasswordCmd carries validated input for resetting a user's password.
// RowVersion is required for optimistic concurrency control: the repository
// performs UPDATE … WHERE row_version = RowVersion and returns apperr.ErrConflict
// when the row has been modified since the form was loaded.
type ResetPasswordCmd struct {
	UserID     string
	Password   string
	RowVersion int
}

// Validate returns an error if the password does not meet policy.
func (c *ResetPasswordCmd) Validate() error {
	if strings.TrimSpace(c.UserID) == "" {
		return fmt.Errorf("user_id is required")
	}
	return validation.PasswordPolicy(c.Password)
}
