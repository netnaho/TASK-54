// Package scheduler provides a lightweight in-process cron runner for report
// definitions. The cron evaluator supports the standard 5-field syntax:
//
//	MIN HOUR DOM MON DOW
//
// Each field accepts: * (any), N (exact), N-M (range), N/S (step from N),
// */S (step from min), N-M/S (step over range), and comma-separated lists.
// No external dependencies are required.
package scheduler

import (
	"strconv"
	"strings"
	"time"
)

// MatchesCron reports whether t falls in the minute window described by the
// 5-field cron expression. Returns false for any malformed expression.
func MatchesCron(expr string, t time.Time) bool {
	fields := strings.Fields(expr)
	if len(fields) != 5 {
		return false
	}
	return matchField(fields[0], t.Minute(), 0, 59) &&
		matchField(fields[1], t.Hour(), 0, 23) &&
		matchField(fields[2], t.Day(), 1, 31) &&
		matchField(fields[3], int(t.Month()), 1, 12) &&
		matchField(fields[4], int(t.Weekday()), 0, 6)
}

// matchField returns true if value is matched by any comma-separated part.
func matchField(expr string, value, min, max int) bool {
	for _, part := range strings.Split(expr, ",") {
		if matchPart(part, value, min, max) {
			return true
		}
	}
	return false
}

// matchPart evaluates a single cron field part (no commas).
// Supported forms: *, N, N-M, */S, N/S, N-M/S.
func matchPart(part string, value, min, max int) bool {
	step := 1
	if idx := strings.Index(part, "/"); idx >= 0 {
		s, err := strconv.Atoi(part[idx+1:])
		if err != nil || s <= 0 {
			return false
		}
		step = s
		part = part[:idx]
	}

	var lo, hi int
	switch {
	case part == "*":
		lo, hi = min, max
	case strings.Contains(part, "-"):
		idx := strings.Index(part, "-")
		var err error
		lo, err = strconv.Atoi(part[:idx])
		if err != nil {
			return false
		}
		hi, err = strconv.Atoi(part[idx+1:])
		if err != nil {
			return false
		}
	default:
		n, err := strconv.Atoi(part)
		if err != nil {
			return false
		}
		lo, hi = n, n
	}

	for v := lo; v <= hi; v += step {
		if v == value {
			return true
		}
	}
	return false
}
