package main

import (
	"testing"
	"time"
)

// Fixed reference time for deterministic tests: Wednesday 2026-09-23 10:00:00
// local time. All calculateNext*RunAt tests pin `now` to this instant instead
// of relying on time.Now(), so results are exact, not "somewhere in the
// future" approximations.
var testNow = time.Date(2026, 9, 23, 10, 0, 0, 0, time.Local)

func TestDayAllowed(t *testing.T) {
	wed := time.Date(2026, 9, 23, 12, 0, 0, 0, time.Local) // Wednesday
	if !dayAllowed(wed, nil) {
		t.Error("empty days list should allow every day")
	}
	if !dayAllowed(wed, []string{}) {
		t.Error("nil-equivalent empty days list should allow every day")
	}
	if !dayAllowed(wed, []string{"Mon", "Wed", "Fri"}) {
		t.Error("Wednesday should be allowed when explicitly listed")
	}
	if dayAllowed(wed, []string{"Mon", "Tue"}) {
		t.Error("Wednesday should NOT be allowed when not listed")
	}
	if !dayAllowed(wed, []string{"wed"}) {
		t.Error("day matching should be case-insensitive")
	}
}

func TestCalculateNextDailyRunAt_NoDayRestriction(t *testing.T) {
	// 10:00 reference, schedule for 14:00 later today.
	got := calculateNextDailyRunAt("14:00", nil, testNow)
	want := time.Date(2026, 9, 23, 14, 0, 0, 0, time.Local).Format(time.RFC3339)
	if got != want {
		t.Errorf("got %s, want %s (today's 14:00 hasn't passed yet)", got, want)
	}

	// Schedule for 08:00 — already passed today, should roll to tomorrow.
	got = calculateNextDailyRunAt("08:00", nil, testNow)
	want = time.Date(2026, 9, 24, 8, 0, 0, 0, time.Local).Format(time.RFC3339)
	if got != want {
		t.Errorf("got %s, want %s (08:00 already passed, should be tomorrow)", got, want)
	}
}

func TestCalculateNextDailyRunAt_DayRestriction(t *testing.T) {
	// testNow is Wednesday. Restrict to Friday only, time already passed
	// today's equivalent doesn't matter here since Wed isn't allowed at all —
	// should skip Wed (today) and Thu, landing on Friday.
	got := calculateNextDailyRunAt("14:00", []string{"Fri"}, testNow)
	want := time.Date(2026, 9, 25, 14, 0, 0, 0, time.Local).Format(time.RFC3339) // Friday
	if got != want {
		t.Errorf("got %s, want %s (should skip to the next allowed Friday)", got, want)
	}
}

func TestCalculateNextDailyRunAt_TodayAllowedButPassed(t *testing.T) {
	// Wednesday allowed, but 08:00 already passed today -> next Wednesday.
	got := calculateNextDailyRunAt("08:00", []string{"Wed"}, testNow)
	want := time.Date(2026, 9, 30, 8, 0, 0, 0, time.Local).Format(time.RFC3339) // next Wednesday
	if got != want {
		t.Errorf("got %s, want %s (only Wed allowed, today's time passed -> next Wed)", got, want)
	}
}

func TestCalculateNextIntervalRunAt_AllDay(t *testing.T) {
	job := ScheduledJob{TriggerMode: "interval", IntervalMinutes: 120, WindowAllDay: true}
	got := calculateNextIntervalRunAt(job, testNow)
	want := testNow.Add(2 * time.Hour).Format(time.RFC3339)
	if got != want {
		t.Errorf("got %s, want %s (all-day: naive now+interval)", got, want)
	}
}

func TestCalculateNextIntervalRunAt_WithinWindow(t *testing.T) {
	// 10:00 + 60min = 11:00, inside a 09:00-17:00 window -> fires at 11:00.
	job := ScheduledJob{
		TriggerMode: "interval", IntervalMinutes: 60,
		WindowStart: "09:00", WindowEnd: "17:00",
	}
	got := calculateNextIntervalRunAt(job, testNow)
	want := testNow.Add(1 * time.Hour).Format(time.RFC3339)
	if got != want {
		t.Errorf("got %s, want %s (candidate falls inside the window)", got, want)
	}
}

func TestCalculateNextIntervalRunAt_BeforeWindowStart(t *testing.T) {
	// now=10:00 + 5min = 10:05, but window doesn't open until 14:00 ->
	// should jump to today's 14:00, not fire early.
	job := ScheduledJob{
		TriggerMode: "interval", IntervalMinutes: 5,
		WindowStart: "14:00", WindowEnd: "18:00",
	}
	got := calculateNextIntervalRunAt(job, testNow)
	want := time.Date(2026, 9, 23, 14, 0, 0, 0, time.Local).Format(time.RFC3339)
	if got != want {
		t.Errorf("got %s, want %s (should wait for today's window to open)", got, want)
	}
}

func TestCalculateNextIntervalRunAt_PastWindowEnd(t *testing.T) {
	// now=10:00 + 600min (10h) = 20:00, past an 08:00-17:00 window ->
	// should roll to TOMORROW's window start, not fire at 20:00 today.
	job := ScheduledJob{
		TriggerMode: "interval", IntervalMinutes: 600,
		WindowStart: "08:00", WindowEnd: "17:00",
	}
	got := calculateNextIntervalRunAt(job, testNow)
	want := time.Date(2026, 9, 24, 8, 0, 0, 0, time.Local).Format(time.RFC3339)
	if got != want {
		t.Errorf("got %s, want %s (past today's window end -> tomorrow's window start)", got, want)
	}
}

func TestCalculateNextIntervalRunAt_WindowPlusDayRestriction(t *testing.T) {
	// testNow is Wednesday. Only Friday allowed, all-day window ->
	// candidate (Wed 12:00) must roll forward to Friday, same time of day.
	job := ScheduledJob{
		TriggerMode: "interval", IntervalMinutes: 120, WindowAllDay: true,
		DaysOfWeek: []string{"Fri"},
	}
	got := calculateNextIntervalRunAt(job, testNow)
	want := testNow.Add(2 * time.Hour).AddDate(0, 0, 2).Format(time.RFC3339) // Wed+2h rolled to Fri
	if got != want {
		t.Errorf("got %s, want %s (should roll forward to the next allowed Friday)", got, want)
	}
}

func TestCalculateNextIntervalRunAt_ZeroIntervalReturnsEmpty(t *testing.T) {
	job := ScheduledJob{TriggerMode: "interval", IntervalMinutes: 0, WindowAllDay: true}
	if got := calculateNextIntervalRunAt(job, testNow); got != "" {
		t.Errorf("expected empty string for a zero/unset interval, got %q", got)
	}
}

func TestCalculateNextRunAt_DispatchesOnTriggerMode(t *testing.T) {
	// Empty TriggerMode (every job saved before this field existed) must
	// behave EXACTLY like "daily" — no migration step should be needed.
	daily := ScheduledJob{ScheduleTime: "14:00"}
	got := calculateNextRunAt(daily, testNow)
	want := calculateNextDailyRunAt("14:00", nil, testNow)
	if got != want {
		t.Errorf("empty TriggerMode diverged from explicit daily: got %s, want %s", got, want)
	}

	interval := ScheduledJob{TriggerMode: "interval", IntervalMinutes: 30, WindowAllDay: true}
	got = calculateNextRunAt(interval, testNow)
	want = calculateNextIntervalRunAt(interval, testNow)
	if got != want {
		t.Errorf("interval dispatch diverged: got %s, want %s", got, want)
	}
}
