package domain

import "time"

// AuditLog is the read model for an immutable audit entry.
// Rows are never updated or deleted through application code.
type AuditLog struct {
	ID             string
	UserID         string
	UserName       string
	Action         string
	EntityType     string
	EntityID       string
	Details        string // JSON object; sensitive values must be masked before insertion
	BeforeSnapshot string // JSON snapshot of entity state before the action (masked)
	AfterSnapshot  string // JSON snapshot of entity state after the action (masked)
	Outcome        string // "success" | "failure" | "error"
	IPAddress      string
	RequestID      string
	OccurredAt     time.Time
}

// ListFilter carries optional query parameters for the audit log list endpoint.
type ListFilter struct {
	EntityType string // filter by entity_type; empty = all
	EntityID   string // filter by entity_id; empty = all
	UserID     string // filter by user_id; empty = all
	ResidentID string // filter to entries linked to this resident across payments, orders, admissions; empty = all
	DateFrom   string // ISO-8601 date YYYY-MM-DD; empty = unbounded
	DateTo     string // ISO-8601 date YYYY-MM-DD; empty = unbounded
	Limit      int    // max rows; 0 = use default
}
