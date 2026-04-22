package domain

import "time"

// MediaRef is a local media asset attached to an exercise.
type MediaRef struct {
	Type     string // "image" | "video"
	Filename string // served at /media/:filename — validated via PathSegment
	Caption  string
}

// ExerciseItem is a single exercise in the facility's library.
type ExerciseItem struct {
	ID                string
	Name              string
	Description       string
	Category          string // "strength" | "balance" | "flexibility" | "cardio" | "cognitive"
	Difficulty        string // "beginner" | "intermediate" | "advanced"
	DurationMinutes   int
	BodyPart          string
	Instructions      string
	Contraindications string
	CoachingPoints    string
	MediaReferences   []MediaRef // decoded from JSON column
	IsActive          bool
	CreatedBy         string
	CreatedAt         time.Time
	UpdatedAt         time.Time

	// Joined
	Tags       []ExerciseTag
	IsFavorite bool
}

// ExerciseTag is a label that can be attached to multiple exercises.
type ExerciseTag struct {
	ID    string
	Name  string
	Color string
}
