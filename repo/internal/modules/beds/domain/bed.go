package domain

import "time"

// Bed represents a physical bed within a ward and room.
type Bed struct {
	ID         string
	Ward       string
	RoomNumber string
	BedLabel   string
	BedType    string // "standard" | "bariatric" | "icu"
	IsActive   bool
	RowVersion int
	CreatedAt  time.Time
	UpdatedAt  time.Time

	// CurrentResidentName is populated by join queries; empty if unoccupied.
	CurrentResidentName string
	IsOccupied          bool
}
