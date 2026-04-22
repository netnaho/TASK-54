package domain

import "time"

// ConfigVersion records a key/value configuration entry with audit trail.
type ConfigVersion struct {
	ID            string
	Namespace     string
	ConfigKey     string
	ConfigValue   string
	Description   string
	ChangedBy     string
	ChangedByName string
	RowVersion    int
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// SnapshotMeta describes a config snapshot file on disk.
type SnapshotMeta struct {
	FileName  string
	CreatedAt time.Time
}
