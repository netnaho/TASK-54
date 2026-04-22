package domain

import "time"

// DiagnosticExport records a request to export system diagnostic data.
type DiagnosticExport struct {
	ID          string
	ExportType  string // "full" | "db_stats" | "logs" | "config"
	TriggeredBy string
	TriggeredByName string
	FilePath    string
	Status      string // "pending" | "complete" | "failed"
	ErrorMessage string
	TriggeredAt time.Time
	CompletedAt *time.Time
	CreatedAt   time.Time
}

// DBStat holds a simple key/value from PRAGMA sqlite_master or similar.
type DBStat struct {
	Key   string
	Value string
}
