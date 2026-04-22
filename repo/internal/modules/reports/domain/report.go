package domain

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"time"
)

// ScheduledReport is a configured recurring report definition.
type ScheduledReport struct {
	ID           string
	Name         string
	ReportType   string
	ScheduleCron string
	OutputFormat string
	Parameters   string // JSON blob for optional filter parameters
	IsActive     bool
	CreatedBy    string
	RowVersion   int
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// ReportRun is a single execution (or attempted execution) of a report.
type ReportRun struct {
	ID                string
	ScheduledReportID string
	ReportType        string
	TriggeredBy       string
	TriggeredByName   string // populated by repository JOIN
	Parameters        string
	OutputPath        string
	OutputFileName    string // derived from OutputPath
	OutputFormat      string
	Status            string // "pending"|"running"|"complete"|"failed"
	ErrorMessage      string
	StartedAt         *time.Time
	CompletedAt       *time.Time
	CreatedAt         time.Time
}

// SetOutputFileName derives the base file name from OutputPath.
func (r *ReportRun) SetOutputFileName() {
	if r.OutputPath != "" {
		r.OutputFileName = filepath.Base(r.OutputPath)
	}
}

// CreateReportCmd carries validated input for creating or updating a report definition.
type CreateReportCmd struct {
	Name         string
	ReportType   string
	ScheduleCron string
	OutputFormat string
	Parameters   string
}

// Validate returns an error if any required field is missing or invalid.
func (c *CreateReportCmd) Validate() error {
	if strings.TrimSpace(c.Name) == "" {
		return fmt.Errorf("name is required")
	}
	valid := map[string]bool{
		"occupancy": true, "service_delivery": true, "finance": true, "audit": true,
	}
	if !valid[c.ReportType] {
		return fmt.Errorf("report_type must be one of: occupancy, service_delivery, finance, audit")
	}
	validFmt := map[string]bool{"csv": true, "xlsx": true}
	if !validFmt[c.OutputFormat] {
		return fmt.Errorf("output_format must be csv or xlsx")
	}
	return nil
}

// ValidReportTypes lists all supported report types for UI select options.
var ValidReportTypes = []string{"occupancy", "service_delivery", "finance", "audit"}

// ValidOutputFormats lists supported file formats.
var ValidOutputFormats = []string{"csv", "xlsx"}

// ReportParams carries optional filter parameters stored as JSON in the Parameters field.
// All fields are optional; zero values mean "no filter".
type ReportParams struct {
	DateFrom   string `json:"date_from"`
	DateTo     string `json:"date_to"`
	ResidentID string `json:"resident_id"`
	UserID     string `json:"user_id"`
	EntityType string `json:"entity_type"`
}

// ParseParams parses the JSON Parameters blob into a ReportParams struct.
// Returns an empty ReportParams on empty or invalid JSON.
func (sr *ScheduledReport) ParseParams() ReportParams {
	if sr.Parameters == "" || sr.Parameters == "{}" {
		return ReportParams{}
	}
	var p ReportParams
	_ = json.Unmarshal([]byte(sr.Parameters), &p)
	return p
}
