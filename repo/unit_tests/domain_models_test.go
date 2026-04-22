package unit_tests

import (
	"testing"
	"time"

	admissiondomain "careops/clinic/internal/modules/admissions/domain"
	bedsdomain "careops/clinic/internal/modules/beds/domain"
	cfgdomain "careops/clinic/internal/modules/config_versions/domain"
	sddomain "careops/clinic/internal/modules/service_delivery/domain"
	usersdomain "careops/clinic/internal/modules/users/domain"
)

// ── Admission domain ──────────────────────────────────────────────────────────

func TestAdmission_IsActive_NilDischargedAt(t *testing.T) {
	a := admissiondomain.Admission{
		ID:           "adm-001",
		ResidentID:   "res-001",
		BedID:        "bed-001",
		DischargedAt: nil,
	}
	if !a.IsActive() {
		t.Error("Admission.IsActive: expected true when DischargedAt is nil")
	}
}

func TestAdmission_IsActive_SetDischargedAt(t *testing.T) {
	discharged := time.Now().UTC()
	a := admissiondomain.Admission{
		ID:           "adm-001",
		ResidentID:   "res-001",
		BedID:        "bed-001",
		DischargedAt: &discharged,
	}
	if a.IsActive() {
		t.Error("Admission.IsActive: expected false when DischargedAt is set")
	}
}

func TestAdmission_IsActive_ZeroTimeDischarge(t *testing.T) {
	// A zero-value time.Time pointer is still a non-nil pointer; considered discharged.
	zero := time.Time{}
	a := admissiondomain.Admission{DischargedAt: &zero}
	if a.IsActive() {
		t.Error("Admission.IsActive: expected false for non-nil zero DischargedAt")
	}
}

// ── Bed domain ────────────────────────────────────────────────────────────────

func TestBed_DefaultFields(t *testing.T) {
	b := bedsdomain.Bed{
		ID:         "bed-001",
		Ward:       "North Wing",
		RoomNumber: "101",
		BedLabel:   "A",
		BedType:    "standard",
		IsActive:   true,
	}
	if b.ID == "" {
		t.Error("Bed: ID must not be empty")
	}
	if !b.IsActive {
		t.Error("Bed: IsActive should be true")
	}
	if b.IsOccupied {
		t.Error("Bed: IsOccupied should default to false")
	}
}

func TestBed_OccupancyStatus(t *testing.T) {
	b := bedsdomain.Bed{
		IsOccupied:          true,
		CurrentResidentName: "Jane Doe",
	}
	if !b.IsOccupied {
		t.Error("Bed: IsOccupied should be true when set")
	}
	if b.CurrentResidentName == "" {
		t.Error("Bed: CurrentResidentName should be populated for occupied bed")
	}
}

// ── User domain ───────────────────────────────────────────────────────────────

func TestUser_IsActive_DefaultsFalse(t *testing.T) {
	u := usersdomain.User{}
	if u.IsActive {
		t.Error("User: IsActive should default to false for zero value")
	}
}

func TestUser_RolesSlice_DefaultsNil(t *testing.T) {
	u := usersdomain.User{ID: "usr-001", Email: "test@example.com"}
	if u.Roles != nil {
		t.Error("User.Roles: should be nil when not populated")
	}
}

// ── Service delivery domain ───────────────────────────────────────────────────

func TestNewAlert_Fields(t *testing.T) {
	a := sddomain.NewAlert{
		ResidentID:  "res-001",
		AlertType:   "fall_risk",
		Severity:    "high",
		Message:     "Patient fell during transfer",
		TriggeredBy: "usr-nurse",
	}
	if a.AlertType != "fall_risk" {
		t.Errorf("NewAlert.AlertType: expected 'fall_risk', got %q", a.AlertType)
	}
	if a.Severity != "high" {
		t.Errorf("NewAlert.Severity: expected 'high', got %q", a.Severity)
	}
}

func TestNewCareCheckpoint_Fields(t *testing.T) {
	cp := sddomain.NewCareCheckpoint{
		ResidentID:     "res-001",
		CheckedBy:      "usr-nurse",
		CheckpointType: "pain_assessment",
		Score:          4,
		Notes:          "Patient reports 3/10 pain",
	}
	if cp.Score < 1 || cp.Score > 5 {
		t.Errorf("NewCareCheckpoint.Score: %d is outside [1,5] range", cp.Score)
	}
	if cp.CheckpointType != "pain_assessment" {
		t.Errorf("NewCareCheckpoint.CheckpointType: expected 'pain_assessment', got %q", cp.CheckpointType)
	}
}

func TestServiceOrder_StatusValues(t *testing.T) {
	for _, status := range []string{"active", "completed", "cancelled"} {
		so := sddomain.ServiceOrder{Status: status}
		if so.Status != status {
			t.Errorf("ServiceOrder.Status: expected %q, got %q", status, so.Status)
		}
	}
}

func TestExecutionRecord_StatusValues(t *testing.T) {
	for _, status := range []string{"completed", "missed", "refused"} {
		er := sddomain.ExecutionRecord{Status: status}
		if er.Status != status {
			t.Errorf("ExecutionRecord.Status: expected %q, got %q", status, er.Status)
		}
	}
}

// ── Config version domain ────────────────────────────────────────────────────

func TestConfigVersion_Fields(t *testing.T) {
	cv := cfgdomain.ConfigVersion{
		ID:          "cfg-001",
		Namespace:   "facility",
		ConfigKey:   "name",
		ConfigValue: "Sunrise Rehab",
		RowVersion:  1,
	}
	if cv.Namespace != "facility" {
		t.Errorf("ConfigVersion.Namespace: expected 'facility', got %q", cv.Namespace)
	}
	if cv.RowVersion != 1 {
		t.Errorf("ConfigVersion.RowVersion: expected 1, got %d", cv.RowVersion)
	}
}

func TestSnapshotMeta_Fields(t *testing.T) {
	ts := time.Now().UTC()
	sm := cfgdomain.SnapshotMeta{
		FileName:  "config_snapshot_20240402_120000.json",
		CreatedAt: ts,
	}
	if sm.FileName == "" {
		t.Error("SnapshotMeta.FileName: must not be empty")
	}
	if sm.CreatedAt.IsZero() {
		t.Error("SnapshotMeta.CreatedAt: must not be zero")
	}
}
