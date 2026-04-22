// Package service implements configuration version management, snapshot
// capture, and one-click rollback for the CareOps application.
//
// What is versioned:
//   - All rows in the config_versions table (namespace/key/value triples).
//   - Snapshots are JSON files written to ConfigSnapPath and tracked in
//     job_history. The file name encodes the timestamp so they sort naturally.
//
// What is NOT versioned:
//   - Runtime secrets (FIELD_ENCRYPT_KEY, DB passwords) — these live in
//     environment variables and are never persisted in config_versions.
//   - Migrations or code — those are in source control.
package service

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	auditsvc "careops/clinic/internal/modules/audit/service"
	audithelper "careops/clinic/internal/shared/audit"
	"careops/clinic/internal/modules/config_versions/domain"
	"careops/clinic/internal/modules/config_versions/repository"
	"careops/clinic/internal/platform/jobs"
)

// ConfigService manages config versioning and rollback.
type ConfigService struct {
	repo      *repository.ConfigRepo
	auditSvc  *auditsvc.AuditService
	jobWriter *jobs.Writer
	snapPath  string
	log       *slog.Logger
}

// New constructs a ConfigService.
func New(
	repo *repository.ConfigRepo,
	auditSvc *auditsvc.AuditService,
	jobWriter *jobs.Writer,
	snapPath string,
	log *slog.Logger,
) *ConfigService {
	return &ConfigService{
		repo:      repo,
		auditSvc:  auditSvc,
		jobWriter: jobWriter,
		snapPath:  snapPath,
		log:       log,
	}
}

// List returns all configuration entries.
func (s *ConfigService) List() ([]domain.ConfigVersion, error) {
	return s.repo.List()
}

// FindByID returns a single entry or an error.
func (s *ConfigService) FindByID(id string) (*domain.ConfigVersion, error) {
	return s.repo.FindByID(id)
}

// Update changes a config value, validates it is non-empty, and records an
// audit log entry.
func (s *ConfigService) Update(id, newValue, changedBy, ipAddress, requestID string, rowVersion int) error {
	if strings.TrimSpace(newValue) == "" {
		return fmt.Errorf("config value must not be empty")
	}

	cv, err := s.repo.FindByID(id)
	if err != nil {
		return fmt.Errorf("config: find %s: %w", id, err)
	}

	before := cv.ConfigValue
	if err := s.repo.UpdateValue(id, newValue, changedBy, rowVersion); err != nil {
		return fmt.Errorf("config: update: %w", err)
	}

	s.auditSvc.Record(auditsvc.Entry{
		UserID:         changedBy,
		Action:         "update",
		EntityType:     "config_versions",
		EntityID:       id,
		Details:        fmt.Sprintf("updated %s/%s", cv.Namespace, cv.ConfigKey),
		BeforeSnapshot: before,
		AfterSnapshot:  newValue,
		IPAddress:      ipAddress,
		RequestID:      requestID,
		Outcome:        "success",
	})
	return nil
}

// TakeSnapshot writes the current config_versions to a JSON file in snapPath.
// Returns the snapshot file path.
func (s *ConfigService) TakeSnapshot(actorID, ipAddress, requestID string) (string, error) {
	entries, err := s.repo.List()
	if err != nil {
		return "", fmt.Errorf("config: list for snapshot: %w", err)
	}

	type snapEntry struct {
		Namespace string `json:"namespace"`
		Key       string `json:"config_key"`
		Value     string `json:"config_value"`
	}
	var snap []snapEntry
	for _, e := range entries {
		snap = append(snap, snapEntry{e.Namespace, e.ConfigKey, e.ConfigValue})
	}

	if err := os.MkdirAll(s.snapPath, 0o755); err != nil {
		return "", fmt.Errorf("config: mkdir snapshots: %w", err)
	}
	ts := time.Now().UTC().Format("20060102_150405")
	fileName := fmt.Sprintf("config_snapshot_%s.json", ts)
	filePath := filepath.Join(s.snapPath, fileName)

	data, err := json.MarshalIndent(snap, "", "  ")
	if err != nil {
		return "", fmt.Errorf("config: marshal snapshot: %w", err)
	}
	if err := os.WriteFile(filePath, data, 0o644); err != nil {
		return "", fmt.Errorf("config: write snapshot: %w", err)
	}

	s.auditSvc.Record(auditsvc.Entry{
		UserID:        actorID,
		Action:        "create",
		EntityType:    "config_snapshot",
		EntityID:      fileName,
		Details:       fmt.Sprintf("captured config snapshot: %s (%d entries)", fileName, len(snap)),
		AfterSnapshot: audithelper.ToJSON(map[string]any{"file_name": fileName, "entry_count": len(snap)}),
		IPAddress:     ipAddress,
		RequestID:     requestID,
		Outcome:       "success",
	})
	_ = s.jobWriter.Record(jobs.Entry{
		JobName: "config_snapshot",
		JobType: "on_demand",
		Status:  jobs.StatusSuccess,
		Detail:  fmt.Sprintf("Snapshot %s written (%d entries)", fileName, len(snap)),
		Actor:   actorID,
	})
	return filePath, nil
}

// ListSnapshots returns the available snapshot files, newest first.
func (s *ConfigService) ListSnapshots() ([]domain.SnapshotMeta, error) {
	entries, err := os.ReadDir(s.snapPath)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("config: list snapshots: %w", err)
	}

	var metas []domain.SnapshotMeta
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		metas = append(metas, domain.SnapshotMeta{
			FileName:  e.Name(),
			CreatedAt: info.ModTime().UTC(),
		})
	}
	sort.Slice(metas, func(i, j int) bool {
		return metas[i].CreatedAt.After(metas[j].CreatedAt)
	})
	return metas, nil
}

// Rollback restores config values from the given snapshot file.
// Validates the snapshot is syntactically correct before applying any change.
func (s *ConfigService) Rollback(fileName, actorID, ipAddress, requestID string) error {
	// Safe path: only allow file names (no directory traversal).
	name := filepath.Base(fileName)
	if !strings.HasSuffix(name, ".json") {
		return fmt.Errorf("invalid snapshot file name")
	}
	filePath := filepath.Join(s.snapPath, name)

	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("config: read snapshot %s: %w", name, err)
	}

	type snapEntry struct {
		Namespace string `json:"namespace"`
		Key       string `json:"config_key"`
		Value     string `json:"config_value"`
	}
	var entries []snapEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return fmt.Errorf("config: parse snapshot %s: %w", name, err)
	}
	if len(entries) == 0 {
		return fmt.Errorf("snapshot %s is empty; refusing rollback", name)
	}

	liveEntries, _ := s.repo.List()
	for _, e := range entries {
		if err := s.repo.Upsert(e.Namespace, e.Key, e.Value, actorID); err != nil {
			return fmt.Errorf("config: rollback upsert %s/%s: %w", e.Namespace, e.Key, err)
		}
	}

	s.auditSvc.Record(auditsvc.Entry{
		UserID:         actorID,
		Action:         "update",
		EntityType:     "config_snapshot",
		EntityID:       name,
		Details:        fmt.Sprintf("rolled back config to snapshot %s (%d entries restored)", name, len(entries)),
		BeforeSnapshot: audithelper.ToJSON(map[string]any{"live_entry_count": len(liveEntries)}),
		AfterSnapshot:  audithelper.ToJSON(map[string]any{"snapshot_file": name, "restored_entry_count": len(entries)}),
		IPAddress:      ipAddress,
		RequestID:      requestID,
		Outcome:        "success",
	})
	_ = s.jobWriter.Record(jobs.Entry{
		JobName: "config_rollback",
		JobType: "on_demand",
		Status:  jobs.StatusSuccess,
		Detail:  fmt.Sprintf("Restored %d config entries from snapshot %s", len(entries), name),
		Actor:   actorID,
	})
	return nil
}
