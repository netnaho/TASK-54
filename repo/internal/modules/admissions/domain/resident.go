package domain

import "time"

// Resident represents a person admitted to or on record at the facility.
type Resident struct {
	ID               string
	MRN              string
	FirstName        string
	LastName         string
	DateOfBirth      string // YYYY-MM-DD
	Gender           string
	PrimaryDiagnosis string
	CareLevel        string // "standard" | "memory" | "skilled"
	RowVersion       int
	CreatedAt        time.Time
	UpdatedAt        time.Time

	// Populated by join queries
	CurrentBed       string // e.g. "East Wing / 101-A"
	AdmissionDate    string
	IsCurrentlyAdmitted bool
}

func (r Resident) FullName() string {
	return r.FirstName + " " + r.LastName
}

// ResidentOption is a lightweight projection used in form select menus.
type ResidentOption struct {
	ID       string
	FullName string
	MRN      string
}
