package repository

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"careops/clinic/internal/modules/users/domain"
	"careops/clinic/internal/shared/apperr"
	"careops/clinic/internal/shared/idgen"
)

var ErrUserNotFound = errors.New("user not found")

// UserRepo provides read and write access to users and roles.
type UserRepo struct {
	db *sql.DB
}

func NewUserRepo(db *sql.DB) *UserRepo {
	return &UserRepo{db: db}
}

// FindByEmail returns a user by email address (case-insensitive due to SQLite COLLATE NOCASE).
func (r *UserRepo) FindByEmail(email string) (*domain.User, error) {
	row := r.db.QueryRow(`
		SELECT id, email, display_name, password_hash, is_active, row_version, created_at, updated_at
		FROM users WHERE email = ? COLLATE NOCASE`, email)
	return scanUser(row)
}

// FindByID returns a user by primary key.
func (r *UserRepo) FindByID(id string) (*domain.User, error) {
	row := r.db.QueryRow(`
		SELECT id, email, display_name, password_hash, is_active, row_version, created_at, updated_at
		FROM users WHERE id = ?`, id)
	return scanUser(row)
}

// List returns all users ordered by display_name.
func (r *UserRepo) List() ([]domain.User, error) {
	rows, err := r.db.Query(`
		SELECT id, email, display_name, password_hash, is_active, row_version, created_at, updated_at
		FROM users ORDER BY display_name`)
	if err != nil {
		return nil, fmt.Errorf("user_repo.List: %w", err)
	}
	defer rows.Close()

	var users []domain.User
	for rows.Next() {
		u, err := scanUserRow(rows)
		if err != nil {
			return nil, err
		}
		users = append(users, *u)
	}
	return users, rows.Err()
}

// ListActive returns all active users ordered by display_name.
func (r *UserRepo) ListActive() ([]domain.User, error) {
	rows, err := r.db.Query(`
		SELECT id, email, display_name, password_hash, is_active, row_version, created_at, updated_at
		FROM users WHERE is_active = 1 ORDER BY display_name`)
	if err != nil {
		return nil, fmt.Errorf("user_repo.ListActive: %w", err)
	}
	defer rows.Close()

	var users []domain.User
	for rows.Next() {
		u, err := scanUserRow(rows)
		if err != nil {
			return nil, err
		}
		users = append(users, *u)
	}
	return users, rows.Err()
}

// FindRoles returns the role names held by the user.
func (r *UserRepo) FindRoles(userID string) ([]string, error) {
	rows, err := r.db.Query(`
		SELECT ro.name FROM roles ro
		INNER JOIN user_roles ur ON ur.role_id = ro.id
		WHERE ur.user_id = ?
		ORDER BY ro.name`, userID)
	if err != nil {
		return nil, fmt.Errorf("user_repo.FindRoles: %w", err)
	}
	defer rows.Close()

	var roles []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		roles = append(roles, name)
	}
	return roles, rows.Err()
}

// ListRoles returns all system roles.
func (r *UserRepo) ListRoles() ([]domain.Role, error) {
	rows, err := r.db.Query(`SELECT id, name, description, created_at, updated_at FROM roles ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("user_repo.ListRoles: %w", err)
	}
	defer rows.Close()

	var roles []domain.Role
	for rows.Next() {
		var ro domain.Role
		var ca, ua string
		if err := rows.Scan(&ro.ID, &ro.Name, &ro.Description, &ca, &ua); err != nil {
			return nil, err
		}
		ro.CreatedAt, _ = time.Parse(time.RFC3339, ca)
		ro.UpdatedAt, _ = time.Parse(time.RFC3339, ua)
		roles = append(roles, ro)
	}
	return roles, rows.Err()
}

// Count returns the total number of users.
func (r *UserRepo) Count() (int, error) {
	var n int
	return n, r.db.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&n)
}

// Create inserts a new user and optionally assigns a role in a single transaction.
// The caller is responsible for hashing the password before passing it in PasswordHash.
func (r *UserRepo) Create(u *domain.User, roleID string) error {
	u.ID = idgen.New()
	now := time.Now().UTC().Format(time.RFC3339)

	tx, err := r.db.Begin()
	if err != nil {
		return fmt.Errorf("user_repo.Create begin: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	_, err = tx.Exec(`
		INSERT INTO users (id, email, display_name, password_hash, is_active, row_version, created_at, updated_at)
		VALUES (?,?,?,?,1,1,?,?)`,
		u.ID, u.Email, u.DisplayName, u.PasswordHash, now, now,
	)
	if err != nil {
		return fmt.Errorf("user_repo.Create: %w", err)
	}

	if roleID != "" {
		_, err = tx.Exec(`INSERT INTO user_roles (user_id, role_id, granted_at) VALUES (?,?,?)`, u.ID, roleID, now)
		if err != nil {
			return fmt.Errorf("user_repo.Create role: %w", err)
		}
	}

	return tx.Commit()
}

// UpdatePasswordHashWithVersion replaces the stored bcrypt hash for a user using
// an optimistic concurrency check. The update is only applied when the stored
// row_version equals rowVersion; if it does not match (concurrent modification
// or stale form), apperr.ErrConflict is returned so the caller can surface a
// 409 to the user. ErrUserNotFound is returned when the user does not exist or
// is inactive.
func (r *UserRepo) UpdatePasswordHashWithVersion(userID, hash string, rowVersion int) error {
	now := time.Now().UTC().Format(time.RFC3339)
	res, err := r.db.Exec(`
		UPDATE users SET password_hash=?, updated_at=?, row_version=row_version+1
		WHERE id=? AND row_version=? AND is_active=1`, hash, now, userID, rowVersion)
	if err != nil {
		return fmt.Errorf("user_repo.UpdatePasswordHashWithVersion: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		// Distinguish a missing/inactive user from a row_version mismatch so callers
		// can return the right error to the handler.
		var current int
		queryErr := r.db.QueryRow(
			`SELECT row_version FROM users WHERE id=? AND is_active=1`, userID,
		).Scan(&current)
		if errors.Is(queryErr, sql.ErrNoRows) {
			return ErrUserNotFound
		}
		if queryErr != nil {
			return fmt.Errorf("user_repo.UpdatePasswordHashWithVersion check: %w", queryErr)
		}
		return &apperr.ConflictError{Entity: "users", ID: userID, CurrentVersion: current}
	}
	return nil
}

// FindRoleByName returns the role ID for the given role name, or ErrUserNotFound.
func (r *UserRepo) FindRoleByName(name string) (string, error) {
	var id string
	err := r.db.QueryRow(`SELECT id FROM roles WHERE name=?`, name).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrUserNotFound
	}
	return id, err
}

// — scan helpers —

func scanUser(row *sql.Row) (*domain.User, error) {
	var u domain.User
	var ca, ua string
	var isActive int
	err := row.Scan(&u.ID, &u.Email, &u.DisplayName, &u.PasswordHash, &isActive, &u.RowVersion, &ca, &ua)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrUserNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("user_repo scan: %w", err)
	}
	u.IsActive = isActive == 1
	u.CreatedAt, _ = time.Parse(time.RFC3339, ca)
	u.UpdatedAt, _ = time.Parse(time.RFC3339, ua)
	return &u, nil
}

func scanUserRow(rows *sql.Rows) (*domain.User, error) {
	var u domain.User
	var ca, ua string
	var isActive int
	if err := rows.Scan(&u.ID, &u.Email, &u.DisplayName, &u.PasswordHash, &isActive, &u.RowVersion, &ca, &ua); err != nil {
		return nil, fmt.Errorf("user_repo scan row: %w", err)
	}
	u.IsActive = isActive == 1
	u.CreatedAt, _ = time.Parse(time.RFC3339, ca)
	u.UpdatedAt, _ = time.Parse(time.RFC3339, ua)
	return &u, nil
}
