package domain

import "time"

// TimelinessThresholdMinutes is the maximum allowed delay (in minutes) between
// a scheduled start and the actual performed_at for a session to be counted on-time.
// This rule is explicit and tested; do not change without updating tests.
const TimelinessThresholdMinutes = 15

// ExecutionMetrics aggregates service delivery performance for a date range.
type ExecutionMetrics struct {
	TotalOrders         int
	ActiveOrders        int
	CompletedOrders     int
	CancelledOrders     int
	TotalExecutions     int
	CompletedExecutions int
	RefusedExecutions   int
	MissedExecutions    int
	// Timeliness: only for records that have a scheduled_start
	ScheduledExecutions int
	OnTimeCount         int
	LateCount           int
	OnTimeRate          float64 // OnTimeCount / ScheduledExecutions; 0 if no scheduled records
	// Completion rate: completed / (completed + refused + missed)
	CompletionRate float64
}

// IsOnTime reports whether an execution was within the timeliness window.
// Returns false when scheduledStart is the zero value (unscheduled session).
func IsOnTime(scheduledStart, performedAt time.Time) bool {
	if scheduledStart.IsZero() {
		return false
	}
	return performedAt.Sub(scheduledStart) <= TimelinessThresholdMinutes*time.Minute
}
