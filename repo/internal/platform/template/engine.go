package template

import (
	"fmt"
	"html/template"
	"strings"
	"time"

	"github.com/gofiber/template/html/v2"
)

// NewEngine returns an html/v2 Fiber template engine configured to load all
// *.html files from templatesDir.  tz is the facility local timezone used by
// the fmtFacility* and toDatetimeLocal helpers; pass time.UTC for tests.
func NewEngine(templatesDir string, tz *time.Location) *html.Engine {
	if tz == nil {
		tz = time.UTC
	}

	engine := html.New(templatesDir, ".html")

	engine.AddFunc("now", func() string {
		return time.Now().UTC().Format(time.RFC3339)
	})

	// fmtDate parses an RFC3339 string and formats as "Jan 2, 2006" in facility time.
	engine.AddFunc("fmtDate", func(s string) string {
		t, err := time.Parse(time.RFC3339, s)
		if err != nil {
			return s
		}
		return t.In(tz).Format("Jan 2, 2006")
	})

	// fmtDateTime parses an RFC3339 string and formats as "Jan 2, 2006 3:04 PM MST".
	engine.AddFunc("fmtDateTime", func(s string) string {
		t, err := time.Parse(time.RFC3339, s)
		if err != nil {
			return s
		}
		return t.In(tz).Format("Jan 2, 2006 3:04 PM MST")
	})

	// fmtFacilityTime formats a time.Time as "3:04 PM MST" in facility local time.
	engine.AddFunc("fmtFacilityTime", func(t time.Time) string {
		if t.IsZero() {
			return "—"
		}
		return t.In(tz).Format("3:04 PM MST")
	})

	// fmtFacilityDateTime formats a time.Time as "Jan 2, 2006 3:04 PM MST".
	engine.AddFunc("fmtFacilityDateTime", func(t time.Time) string {
		if t.IsZero() {
			return "—"
		}
		return t.In(tz).Format("Jan 2, 2006 3:04 PM MST")
	})

	// fmtFacilityDate formats a time.Time as "Jan 2, 2006" in facility local time.
	engine.AddFunc("fmtFacilityDate", func(t time.Time) string {
		if t.IsZero() {
			return "—"
		}
		return t.In(tz).Format("Jan 2, 2006")
	})

	// toDatetimeLocal converts a time.Time to the HTML datetime-local input format
	// ("2006-01-02T15:04") in facility local time.  Used for form value= attrs.
	engine.AddFunc("toDatetimeLocal", func(t time.Time) string {
		if t.IsZero() {
			return ""
		}
		return t.In(tz).Format("2006-01-02T15:04")
	})

	// timelineOffset returns the CSS percentage left offset for a time on a
	// day-view timeline.  dayStartHour: first hour shown (e.g. 8 for 8am).
	// dayMinutes: total minutes the timeline spans (e.g. 600 for 10 hours).
	engine.AddFunc("timelineOffset", func(t time.Time, dayStartHour, dayMinutes int) string {
		if t.IsZero() {
			return "0"
		}
		local := t.In(tz)
		startOfSlot := time.Date(local.Year(), local.Month(), local.Day(),
			dayStartHour, 0, 0, 0, tz)
		offset := local.Sub(startOfSlot).Minutes()
		if offset < 0 {
			offset = 0
		}
		if float64(dayMinutes) == 0 {
			return "0"
		}
		pct := offset / float64(dayMinutes) * 100
		if pct > 100 {
			pct = 100
		}
		return fmt.Sprintf("%.2f%%", pct)
	})

	// timelineWidth returns the CSS percentage width for a session block.
	engine.AddFunc("timelineWidth", func(start, end time.Time, dayMinutes int) string {
		if start.IsZero() || end.IsZero() || dayMinutes == 0 {
			return "0%"
		}
		dur := end.Sub(start).Minutes()
		if dur <= 0 {
			return "0%"
		}
		pct := dur / float64(dayMinutes) * 100
		if pct > 100 {
			pct = 100
		}
		return fmt.Sprintf("%.2f%%", pct)
	})

	engine.AddFunc("hasRole", func(roles []string, role string) bool {
		for _, r := range roles {
			if r == role {
				return true
			}
		}
		return false
	})

	engine.AddFunc("joinStr", func(items []string, sep string) string {
		return strings.Join(items, sep)
	})

	engine.AddFunc("pct", func(num, denom int) string {
		if denom == 0 {
			return "0%"
		}
		return fmt.Sprintf("%.0f%%", float64(num)/float64(denom)*100)
	})

	engine.AddFunc("safeHTML", func(s string) template.HTML {
		return template.HTML(s) //nolint:gosec
	})

	engine.AddFunc("divCents", func(cents int) string {
		return fmt.Sprintf("%d.%02d", cents/100, cents%100)
	})

	engine.AddFunc("add", func(a, b int) int { return a + b })
	engine.AddFunc("sub", func(a, b int) int { return a - b })

	// dict builds a map[string]any from alternating key/value pairs.
	// Enables passing named data to sub-templates: {{template "foo" (dict "K" v)}}.
	engine.AddFunc("dict", func(pairs ...any) (map[string]any, error) {
		if len(pairs)%2 != 0 {
			return nil, fmt.Errorf("dict requires an even number of arguments")
		}
		m := make(map[string]any, len(pairs)/2)
		for i := 0; i < len(pairs); i += 2 {
			key, ok := pairs[i].(string)
			if !ok {
				return nil, fmt.Errorf("dict keys must be strings")
			}
			m[key] = pairs[i+1]
		}
		return m, nil
	})

	// seq returns a slice of ints from start to end (inclusive).
	engine.AddFunc("seq", func(start, end int) []int {
		s := make([]int, 0, end-start+1)
		for i := start; i <= end; i++ {
			s = append(s, i)
		}
		return s
	})

	// hourOffset returns the CSS percentage left for a given hour mark on the timeline.
	// hour is in facility local time; dayStartHour and dayMinutes define the visible range.
	engine.AddFunc("hourOffset", func(hour, dayStartHour, dayMinutes int) string {
		if dayMinutes == 0 {
			return "0"
		}
		mins := float64((hour - dayStartHour) * 60)
		if mins < 0 {
			mins = 0
		}
		pct := mins / float64(dayMinutes) * 100
		return fmt.Sprintf("%.2f%%", pct)
	})

	engine.AddFunc("fmtDateShort", func(s string) string {
		if t, err := time.Parse(time.RFC3339, s); err == nil {
			return t.In(tz).Format("Jan 2, 2006")
		}
		if t, err := time.Parse("2006-01-02", s); err == nil {
			return t.Format("Jan 2, 2006")
		}
		return s
	})

	engine.AddFunc("timeAgo", func(t time.Time) string {
		if t.IsZero() {
			return "—"
		}
		d := time.Since(t)
		switch {
		case d < time.Minute:
			return "just now"
		case d < time.Hour:
			return fmt.Sprintf("%dm ago", int(d.Minutes()))
		case d < 24*time.Hour:
			return fmt.Sprintf("%dh ago", int(d.Hours()))
		default:
			return fmt.Sprintf("%dd ago", int(d.Hours()/24))
		}
	})

	engine.AddFunc("pctF", func(f float64) string {
		return fmt.Sprintf("%.0f%%", f*100)
	})

	engine.AddFunc("scoreColor", func(score int) string {
		switch {
		case score >= 4:
			return "green"
		case score == 3:
			return "orange"
		default:
			return "red"
		}
	})

	return engine
}
