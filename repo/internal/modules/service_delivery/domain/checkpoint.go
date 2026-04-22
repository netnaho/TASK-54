package domain

import "time"

// CareCheckpoint records a quality-of-care observation attached to a resident.
type CareCheckpoint struct {
	ID             string
	ResidentID     string
	CheckedBy      string
	CheckpointType string // "pain_assessment" | "mobility_check" | "skin_integrity" | "wound_assessment" | "fall_risk_screen" | "functional_status" | "nutrition_check" | "engagement_check"
	Score          int    // 1 (poor) – 5 (excellent)
	Notes          string
	CheckedAt      time.Time
	CreatedAt      time.Time

	// Joined
	ResidentName  string
	CheckedByName string
}

// NewCareCheckpoint is the input for creating a checkpoint record.
type NewCareCheckpoint struct {
	ResidentID     string
	CheckedBy      string
	CheckpointType string
	Score          int
	Notes          string
}
