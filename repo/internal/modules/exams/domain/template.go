package domain

import (
	"fmt"
	"strings"
	"time"
)

// ExamTemplate defines a reusable competency examination blueprint.
// duration_minutes and window_start/end were added in migration 0005.
type ExamTemplate struct {
	ID              string
	Title           string
	Description     string
	RoleScope       string
	PassingScore    int
	DurationMinutes int
	// WindowStart and WindowEnd are "HH:MM" in facility local time (FACILITY_TIMEZONE).
	// They define the earliest and latest allowed time for any session using this template.
	WindowStart     string
	WindowEnd       string
	TemplateVersion int
	IsActive        bool
	CreatedBy       string
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// CreateTemplate holds the fields required to create a new exam template.
type CreateTemplate struct {
	Title           string
	Description     string
	RoleScope       string
	PassingScore    int
	DurationMinutes int
	WindowStart     string
	WindowEnd       string
}

// UpdateTemplate holds editable fields.  Every successful update bumps TemplateVersion.
type UpdateTemplate struct {
	Title           string
	Description     string
	DurationMinutes int
	WindowStart     string
	WindowEnd       string
	PassingScore    int
}

// Validate checks that a CreateTemplate has acceptable values.
func (c *CreateTemplate) Validate() error {
	if strings.TrimSpace(c.Title) == "" {
		return fmt.Errorf("title is required")
	}
	if strings.TrimSpace(c.RoleScope) == "" {
		return fmt.Errorf("role scope is required")
	}
	if c.PassingScore < 1 || c.PassingScore > 100 {
		return fmt.Errorf("passing score must be between 1 and 100")
	}
	if c.DurationMinutes < 5 || c.DurationMinutes > 480 {
		return fmt.Errorf("duration must be between 5 and 480 minutes")
	}
	if err := validateWindowTime(c.WindowStart, "window_start"); err != nil {
		return err
	}
	if err := validateWindowTime(c.WindowEnd, "window_end"); err != nil {
		return err
	}
	ws, _ := parseHHMM(c.WindowStart)
	we, _ := parseHHMM(c.WindowEnd)
	if !we.After(ws) {
		return fmt.Errorf("window_end must be after window_start")
	}
	// Verify the window is at least as wide as the duration.
	windowMinutes := int(we.Sub(ws).Minutes())
	if windowMinutes < c.DurationMinutes {
		return fmt.Errorf("window (%d min) must be at least as wide as duration (%d min)", windowMinutes, c.DurationMinutes)
	}
	return nil
}

// Validate checks that an UpdateTemplate has acceptable values.
func (u *UpdateTemplate) Validate() error {
	if strings.TrimSpace(u.Title) == "" {
		return fmt.Errorf("title is required")
	}
	if u.PassingScore < 1 || u.PassingScore > 100 {
		return fmt.Errorf("passing score must be between 1 and 100")
	}
	if u.DurationMinutes < 5 || u.DurationMinutes > 480 {
		return fmt.Errorf("duration must be between 5 and 480 minutes")
	}
	if err := validateWindowTime(u.WindowStart, "window_start"); err != nil {
		return err
	}
	if err := validateWindowTime(u.WindowEnd, "window_end"); err != nil {
		return err
	}
	ws, _ := parseHHMM(u.WindowStart)
	we, _ := parseHHMM(u.WindowEnd)
	if !we.After(ws) {
		return fmt.Errorf("window_end must be after window_start")
	}
	windowMinutes := int(we.Sub(ws).Minutes())
	if windowMinutes < u.DurationMinutes {
		return fmt.Errorf("window (%d min) must be at least as wide as duration (%d min)", windowMinutes, u.DurationMinutes)
	}
	return nil
}

// WindowStartUTC returns the window_start for the given date in UTC.
// The input date is used to compute the correct UTC offset for tz.
func (t *ExamTemplate) WindowStartUTC(date time.Time, tz *time.Location) (time.Time, error) {
	return windowTimeUTC(t.WindowStart, date, tz)
}

// WindowEndUTC returns the window_end for the given date in UTC.
func (t *ExamTemplate) WindowEndUTC(date time.Time, tz *time.Location) (time.Time, error) {
	return windowTimeUTC(t.WindowEnd, date, tz)
}

// windowTimeUTC converts an "HH:MM" facility-local time on a given date to UTC.
func windowTimeUTC(hhmm string, date time.Time, tz *time.Location) (time.Time, error) {
	hm, err := parseHHMM(hhmm)
	if err != nil {
		return time.Time{}, err
	}
	local := date.In(tz)
	t := time.Date(local.Year(), local.Month(), local.Day(),
		hm.Hour(), hm.Minute(), 0, 0, tz)
	return t.UTC(), nil
}

// validateWindowTime returns an error if s is not a valid "HH:MM" string.
func validateWindowTime(s, field string) error {
	if _, err := parseHHMM(s); err != nil {
		return fmt.Errorf("%s must be HH:MM (e.g. \"08:00\"), got %q", field, s)
	}
	return nil
}

// parseHHMM parses "HH:MM" into a time.Time on the zero date (used only for hour/minute).
// The string must be exactly 5 characters in the format HH:MM (e.g. "08:00", "17:30").
func parseHHMM(s string) (time.Time, error) {
	if len(s) != 5 || s[2] != ':' {
		return time.Time{}, fmt.Errorf("invalid HH:MM value %q: must be exactly HH:MM", s)
	}
	t, err := time.Parse("15:04", s)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid HH:MM value %q: %w", s, err)
	}
	return t, nil
}
