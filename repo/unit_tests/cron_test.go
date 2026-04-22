package unit_tests

import (
	"testing"
	"time"

	"careops/clinic/internal/modules/reports/scheduler"
)

func cronTime(min, hour, dom, month, dow int) time.Time {
	// Build a time with given minute, hour, day, month, weekday.
	// We fix to a known week: 2024-01-07 is a Sunday (weekday 0).
	// 2024-01-08 is Monday (1), ..., 2024-01-13 is Saturday (6).
	base := time.Date(2024, 1, 7, 0, 0, 0, 0, time.UTC) // Sunday
	t := time.Date(
		base.Year()+0,
		time.Month(month),
		dom,
		hour, min, 0, 0,
		time.UTC,
	)
	_ = dow
	return t
}

func TestMatchesCron_Wildcard(t *testing.T) {
	now := time.Now()
	if !scheduler.MatchesCron("* * * * *", now) {
		t.Error("* * * * * should match any time")
	}
}

func TestMatchesCron_ExactMatch(t *testing.T) {
	// "30 6 15 3 *" should match 06:30 on March 15 of any weekday
	ts := time.Date(2024, 3, 15, 6, 30, 0, 0, time.UTC) // Friday
	if !scheduler.MatchesCron("30 6 15 3 *", ts) {
		t.Errorf("30 6 15 3 * should match 2024-03-15 06:30 UTC")
	}
}

func TestMatchesCron_ExactNoMatch(t *testing.T) {
	ts := time.Date(2024, 3, 15, 6, 31, 0, 0, time.UTC)
	if scheduler.MatchesCron("30 6 15 3 *", ts) {
		t.Error("30 6 15 3 * should not match 06:31")
	}
}

func TestMatchesCron_DailyAtMidnight(t *testing.T) {
	ts := time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC)
	if !scheduler.MatchesCron("0 0 * * *", ts) {
		t.Error("0 0 * * * should match midnight")
	}
	ts2 := time.Date(2024, 6, 1, 0, 1, 0, 0, time.UTC)
	if scheduler.MatchesCron("0 0 * * *", ts2) {
		t.Error("0 0 * * * should not match 00:01")
	}
}

func TestMatchesCron_WeekdayOnly(t *testing.T) {
	// "0 9 * * 1" — every Monday at 09:00
	monday := time.Date(2024, 1, 8, 9, 0, 0, 0, time.UTC) // 2024-01-08 is Monday
	if !scheduler.MatchesCron("0 9 * * 1", monday) {
		t.Error("0 9 * * 1 should match Monday 09:00")
	}
	tuesday := time.Date(2024, 1, 9, 9, 0, 0, 0, time.UTC)
	if scheduler.MatchesCron("0 9 * * 1", tuesday) {
		t.Error("0 9 * * 1 should not match Tuesday")
	}
}

func TestMatchesCron_HourRange(t *testing.T) {
	// "0 9-17 * * *" — on the hour between 9 and 17
	ts9 := time.Date(2024, 1, 1, 9, 0, 0, 0, time.UTC)
	ts17 := time.Date(2024, 1, 1, 17, 0, 0, 0, time.UTC)
	ts18 := time.Date(2024, 1, 1, 18, 0, 0, 0, time.UTC)
	ts8 := time.Date(2024, 1, 1, 8, 0, 0, 0, time.UTC)
	if !scheduler.MatchesCron("0 9-17 * * *", ts9) {
		t.Error("should match hour 9")
	}
	if !scheduler.MatchesCron("0 9-17 * * *", ts17) {
		t.Error("should match hour 17")
	}
	if scheduler.MatchesCron("0 9-17 * * *", ts18) {
		t.Error("should not match hour 18")
	}
	if scheduler.MatchesCron("0 9-17 * * *", ts8) {
		t.Error("should not match hour 8")
	}
}

func TestMatchesCron_Step(t *testing.T) {
	// "*/15 * * * *" — every 15 minutes
	for _, min := range []int{0, 15, 30, 45} {
		ts := time.Date(2024, 1, 1, 10, min, 0, 0, time.UTC)
		if !scheduler.MatchesCron("*/15 * * * *", ts) {
			t.Errorf("*/15 should match minute %d", min)
		}
	}
	for _, min := range []int{1, 14, 16, 29, 31, 44, 46} {
		ts := time.Date(2024, 1, 1, 10, min, 0, 0, time.UTC)
		if scheduler.MatchesCron("*/15 * * * *", ts) {
			t.Errorf("*/15 should not match minute %d", min)
		}
	}
}

func TestMatchesCron_List(t *testing.T) {
	// "0 8,12,18 * * *" — at 08:00, 12:00, and 18:00
	for _, hr := range []int{8, 12, 18} {
		ts := time.Date(2024, 1, 1, hr, 0, 0, 0, time.UTC)
		if !scheduler.MatchesCron("0 8,12,18 * * *", ts) {
			t.Errorf("list 8,12,18 should match hour %d", hr)
		}
	}
	for _, hr := range []int{7, 9, 11, 13, 17, 19} {
		ts := time.Date(2024, 1, 1, hr, 0, 0, 0, time.UTC)
		if scheduler.MatchesCron("0 8,12,18 * * *", ts) {
			t.Errorf("list 8,12,18 should not match hour %d", hr)
		}
	}
}

func TestMatchesCron_MalformedExpression(t *testing.T) {
	bad := []string{
		"",
		"* * *",
		"* * * * * *",
		"not a cron",
	}
	now := time.Now()
	for _, expr := range bad {
		if scheduler.MatchesCron(expr, now) {
			t.Errorf("malformed expression %q should not match", expr)
		}
	}
}

func TestMatchesCron_WeeklyMondayMorning(t *testing.T) {
	// "0 6 * * 1" — every Monday at 06:00
	monday0600 := time.Date(2024, 1, 8, 6, 0, 0, 0, time.UTC)
	if !scheduler.MatchesCron("0 6 * * 1", monday0600) {
		t.Error("0 6 * * 1 should match Monday 06:00")
	}
}
