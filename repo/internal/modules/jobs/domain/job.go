package domain

import "time"

// JobHistory records the outcome of a scheduled or on-demand background job.
type JobHistory struct {
	ID            string
	JobName       string
	JobType       string // "scheduled" | "on_demand"
	Status        string // "success" | "failed" | "skipped"
	DurationMs    int
	Detail        string
	Actor         string
	CorrelationID string
	RanAt         time.Time
	CreatedAt     time.Time
}
