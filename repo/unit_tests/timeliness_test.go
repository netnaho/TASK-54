package unit_tests

import (
	"testing"
	"time"

	sddomain "careops/clinic/internal/modules/service_delivery/domain"
)

func TestIsOnTime_OnTime(t *testing.T) {
	scheduled := time.Date(2024, 1, 15, 9, 0, 0, 0, time.UTC)
	performed := scheduled.Add(10 * time.Minute) // 10 min early — within threshold
	if !sddomain.IsOnTime(scheduled, performed) {
		t.Error("expected on-time for 10-minute delta")
	}
}

func TestIsOnTime_ExactThreshold(t *testing.T) {
	scheduled := time.Date(2024, 1, 15, 9, 0, 0, 0, time.UTC)
	performed := scheduled.Add(sddomain.TimelinessThresholdMinutes * time.Minute)
	if !sddomain.IsOnTime(scheduled, performed) {
		t.Error("expected on-time at exact threshold boundary")
	}
}

func TestIsOnTime_Late(t *testing.T) {
	scheduled := time.Date(2024, 1, 15, 9, 0, 0, 0, time.UTC)
	performed := scheduled.Add(sddomain.TimelinessThresholdMinutes*time.Minute + time.Second)
	if sddomain.IsOnTime(scheduled, performed) {
		t.Error("expected late for threshold+1s delta")
	}
}

func TestIsOnTime_EarlyIsOnTime(t *testing.T) {
	scheduled := time.Date(2024, 1, 15, 9, 0, 0, 0, time.UTC)
	performed := scheduled.Add(-5 * time.Minute) // performed before scheduled — still on-time
	if !sddomain.IsOnTime(scheduled, performed) {
		t.Error("expected on-time when performed before scheduled start")
	}
}

func TestIsOnTime_ZeroScheduled(t *testing.T) {
	// Unscheduled session — zero scheduledStart must return false
	if sddomain.IsOnTime(time.Time{}, time.Now()) {
		t.Error("expected false for zero scheduledStart (unscheduled session)")
	}
}

func TestTimelinessThresholdConstant(t *testing.T) {
	if sddomain.TimelinessThresholdMinutes != 15 {
		t.Errorf("TimelinessThresholdMinutes must be 15, got %d", sddomain.TimelinessThresholdMinutes)
	}
}
