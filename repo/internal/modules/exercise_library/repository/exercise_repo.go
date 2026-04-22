package repository

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"careops/clinic/internal/modules/exercise_library/domain"
)

// ExerciseFilter is the set of optional filters for the exercise listing.
type ExerciseFilter struct {
	Category      string
	Difficulty    string
	TagID         string
	BodyPart      string
	Query         string // searches name, description, contraindications and coaching_points
	FavoritesOnly bool
}

type ExerciseRepo struct{ db *sql.DB }

func NewExerciseRepo(db *sql.DB) *ExerciseRepo { return &ExerciseRepo{db: db} }

// List returns active exercises matching the filter for the given user (for favorites).
func (r *ExerciseRepo) List(f ExerciseFilter, userID string) ([]domain.ExerciseItem, error) {
	query := `
		SELECT ei.id, ei.name, ei.description, ei.category, ei.difficulty,
		       ei.duration_minutes, COALESCE(ei.body_part,''),
		       COALESCE(ei.instructions,''), COALESCE(ei.contraindications,''),
		       COALESCE(ei.coaching_points,''), COALESCE(ei.media_references,'[]'),
		       ei.is_active, ei.created_by, ei.created_at, ei.updated_at,
		       CASE WHEN ef.exercise_id IS NOT NULL THEN 1 ELSE 0 END AS is_favorite
		FROM exercise_items ei
		LEFT JOIN exercise_favorites ef ON ef.exercise_id = ei.id AND ef.user_id = ?
		WHERE ei.is_active = 1`

	args := []any{userID}

	if f.Category != "" {
		query += " AND ei.category = ?"
		args = append(args, f.Category)
	}
	if f.Difficulty != "" {
		query += " AND ei.difficulty = ?"
		args = append(args, f.Difficulty)
	}
	if f.BodyPart != "" {
		query += " AND ei.body_part = ?"
		args = append(args, f.BodyPart)
	}
	if f.Query != "" {
		query += " AND (ei.name LIKE ? OR ei.description LIKE ? OR ei.contraindications LIKE ? OR ei.coaching_points LIKE ?)"
		like := "%" + f.Query + "%"
		args = append(args, like, like, like, like)
	}
	if f.TagID != "" {
		query += " AND EXISTS (SELECT 1 FROM exercise_item_tags eit WHERE eit.exercise_id = ei.id AND eit.tag_id = ?)"
		args = append(args, f.TagID)
	}
	if f.FavoritesOnly {
		query += " AND ef.exercise_id IS NOT NULL"
	}
	query += " ORDER BY ei.category, ei.name"

	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("exercise_repo.List: %w", err)
	}
	defer rows.Close()

	var items []domain.ExerciseItem
	for rows.Next() {
		ex, err := scanExerciseRow(rows)
		if err != nil {
			return nil, err
		}
		tags, err := r.tagsForExercise(ex.ID)
		if err != nil {
			return nil, err
		}
		ex.Tags = tags
		items = append(items, ex)
	}
	return items, rows.Err()
}

// FindByID returns the exercise with full detail, or nil if not found.
func (r *ExerciseRepo) FindByID(id, userID string) (*domain.ExerciseItem, error) {
	row := r.db.QueryRow(`
		SELECT ei.id, ei.name, ei.description, ei.category, ei.difficulty,
		       ei.duration_minutes, COALESCE(ei.body_part,''),
		       COALESCE(ei.instructions,''), COALESCE(ei.contraindications,''),
		       COALESCE(ei.coaching_points,''), COALESCE(ei.media_references,'[]'),
		       ei.is_active, ei.created_by, ei.created_at, ei.updated_at,
		       CASE WHEN ef.exercise_id IS NOT NULL THEN 1 ELSE 0 END
		FROM exercise_items ei
		LEFT JOIN exercise_favorites ef ON ef.exercise_id = ei.id AND ef.user_id = ?
		WHERE ei.id = ?`, userID, id)

	ex, err := scanExerciseRow(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("exercise_repo.FindByID: %w", err)
	}
	ex.Tags, err = r.tagsForExercise(ex.ID)
	if err != nil {
		return nil, err
	}
	return &ex, nil
}

// ToggleFavorite adds the favorite if it does not exist, removes it if it does.
// Returns true if the exercise is now a favorite, false if removed.
func (r *ExerciseRepo) ToggleFavorite(userID, exerciseID string) (bool, error) {
	var exists int
	err := r.db.QueryRow(
		`SELECT COUNT(*) FROM exercise_favorites WHERE user_id = ? AND exercise_id = ?`,
		userID, exerciseID,
	).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("exercise_repo.ToggleFavorite check: %w", err)
	}
	if exists > 0 {
		_, err = r.db.Exec(
			`DELETE FROM exercise_favorites WHERE user_id = ? AND exercise_id = ?`,
			userID, exerciseID,
		)
		return false, err
	}
	now := time.Now().UTC().Format(time.RFC3339)
	_, err = r.db.Exec(
		`INSERT INTO exercise_favorites (user_id, exercise_id, created_at) VALUES (?, ?, ?)`,
		userID, exerciseID, now,
	)
	return true, err
}

// AllTags returns every tag, ordered by name.
func (r *ExerciseRepo) AllTags() ([]domain.ExerciseTag, error) {
	rows, err := r.db.Query(`SELECT id, name, color FROM exercise_tags ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("exercise_repo.AllTags: %w", err)
	}
	defer rows.Close()
	var tags []domain.ExerciseTag
	for rows.Next() {
		var t domain.ExerciseTag
		if err := rows.Scan(&t.ID, &t.Name, &t.Color); err != nil {
			return nil, err
		}
		tags = append(tags, t)
	}
	return tags, rows.Err()
}

// AllBodyParts returns distinct non-empty body_part values from active exercises.
func (r *ExerciseRepo) AllBodyParts() ([]string, error) {
	rows, err := r.db.Query(
		`SELECT DISTINCT body_part FROM exercise_items WHERE is_active = 1 AND body_part IS NOT NULL AND body_part != '' ORDER BY body_part`)
	if err != nil {
		return nil, fmt.Errorf("exercise_repo.AllBodyParts: %w", err)
	}
	defer rows.Close()
	var parts []string
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			return nil, err
		}
		parts = append(parts, p)
	}
	return parts, rows.Err()
}

// AllCategories returns distinct active categories.
func (r *ExerciseRepo) AllCategories() ([]string, error) {
	rows, err := r.db.Query(
		`SELECT DISTINCT category FROM exercise_items WHERE is_active = 1 ORDER BY category`)
	if err != nil {
		return nil, fmt.Errorf("exercise_repo.AllCategories: %w", err)
	}
	defer rows.Close()
	var cats []string
	for rows.Next() {
		var c string
		if err := rows.Scan(&c); err != nil {
			return nil, err
		}
		cats = append(cats, c)
	}
	return cats, rows.Err()
}

// scanner is satisfied by both *sql.Row and *sql.Rows for scanExerciseRow.
type scanner interface {
	Scan(dest ...any) error
}

func scanExerciseRow(s scanner) (domain.ExerciseItem, error) {
	var ex domain.ExerciseItem
	var ca, ua, mediaJSON string
	var isActive, isFav int
	if err := s.Scan(
		&ex.ID, &ex.Name, &ex.Description, &ex.Category, &ex.Difficulty,
		&ex.DurationMinutes, &ex.BodyPart,
		&ex.Instructions, &ex.Contraindications, &ex.CoachingPoints, &mediaJSON,
		&isActive, &ex.CreatedBy, &ca, &ua, &isFav,
	); err != nil {
		return ex, err
	}
	ex.IsActive = isActive == 1
	ex.IsFavorite = isFav == 1
	ex.CreatedAt, _ = time.Parse(time.RFC3339, ca)
	ex.UpdatedAt, _ = time.Parse(time.RFC3339, ua)
	if mediaJSON != "" && mediaJSON != "[]" {
		_ = json.Unmarshal([]byte(mediaJSON), &ex.MediaReferences)
	}
	return ex, nil
}

func (r *ExerciseRepo) tagsForExercise(exerciseID string) ([]domain.ExerciseTag, error) {
	rows, err := r.db.Query(`
		SELECT et.id, et.name, et.color FROM exercise_tags et
		INNER JOIN exercise_item_tags eit ON eit.tag_id = et.id
		WHERE eit.exercise_id = ?`, exerciseID)
	if err != nil {
		return nil, fmt.Errorf("exercise_repo.tagsForExercise: %w", err)
	}
	defer rows.Close()
	var tags []domain.ExerciseTag
	for rows.Next() {
		var t domain.ExerciseTag
		if err := rows.Scan(&t.ID, &t.Name, &t.Color); err != nil {
			return nil, err
		}
		tags = append(tags, t)
	}
	return tags, rows.Err()
}
