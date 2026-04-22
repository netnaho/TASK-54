package domain

import "time"

// Admission records a resident being placed in a specific bed.
type Admission struct {
	ID              string
	ResidentID      string
	BedID           string
	AdmittedAt      time.Time
	DischargedAt    *time.Time // nil while active
	DischargeReason string
	AdmittedBy      string
	RowVersion      int
	CreatedAt       time.Time
	UpdatedAt       time.Time

	// Joined fields
	ResidentName string
	BedLocation  string // "Ward / Room-Label"
	AdmittedByName string
}

func (a Admission) IsActive() bool {
	return a.DischargedAt == nil
}
