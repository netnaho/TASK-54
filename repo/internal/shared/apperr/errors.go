// Package apperr defines typed sentinel errors shared across layers.
// Handlers map these to HTTP status codes; services return them without
// importing HTTP concerns.
package apperr

import (
	"errors"
	"fmt"
)

// Sentinel errors checked with errors.Is.
var (
	ErrNotFound     = errors.New("not found")
	ErrUnauthorized = errors.New("unauthorized")
	ErrForbidden    = errors.New("forbidden")
	ErrConflict     = errors.New("optimistic lock conflict")
	ErrDuplicate    = errors.New("duplicate request")
	ErrValidation   = errors.New("validation error")
)

// ConflictError is returned when an optimistic lock check fails.
// It carries the current version so the caller can reload the record.
type ConflictError struct {
	Entity         string
	ID             string
	CurrentVersion int
}

func (e *ConflictError) Error() string {
	return fmt.Sprintf("conflict: %s/%s was modified elsewhere (current version %d)",
		e.Entity, e.ID, e.CurrentVersion)
}

func (e *ConflictError) Unwrap() error { return ErrConflict }

// ValidationError carries a single field-level validation failure.
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("validation: %s: %s", e.Field, e.Message)
}

func (e *ValidationError) Unwrap() error { return ErrValidation }
