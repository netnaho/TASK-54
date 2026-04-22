package domain

import "time"

// ServiceOrder is a clinician's instruction to provide a specific therapy or care service.
type ServiceOrder struct {
	ID           string
	ResidentID   string
	OrderedBy    string
	ServiceType  string // "PT" | "OT" | "ST" | "nursing" | "aide"
	Frequency    string
	Instructions string
	Status       string // "active" | "completed" | "cancelled"
	StartDate    string
	EndDate      string
	RowVersion   int
	CreatedAt    time.Time
	UpdatedAt    time.Time

	// Joined
	ResidentName  string
	OrderedByName string
}

// ExecutionRecord documents one completed (or missed) session for a service order.
type ExecutionRecord struct {
	ID              string
	ServiceOrderID  string
	PerformedBy     string
	ScheduledStart  time.Time // zero if unscheduled
	PerformedAt     time.Time
	DurationMinutes int
	Notes           string
	Status          string // "completed" | "missed" | "refused"
	CreatedAt       time.Time

	// Joined
	PerformedByName string
	ResidentName    string
	ServiceType     string
}

// Alert represents a triggered care alert that may require follow-up.
type Alert struct {
	ID         string
	ResidentID string
	AlertType  string // "fall_risk" | "medication" | "vitals" | "discharge" | "wound_change"
	Severity   string // "low" | "medium" | "high" | "critical"
	Message    string
	IsResolved bool
	TriggeredAt time.Time

	// Joined
	ResidentName string
}

// NewAlert is the input for creating an alert event.
type NewAlert struct {
	ResidentID  string
	AlertType   string
	Severity    string
	Message     string
	TriggeredBy string
}
