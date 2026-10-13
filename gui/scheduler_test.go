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

// Sets saved before schedules existed (TriggerMode and its fields) are read
// as one schedule. Daily keeps its times; a repeat now runs on the clock from
// the window start (or midnight) instead of counting from the last run.
func TestLegacyTriggerNextRun(t *testing.T) {
	at := func(d, h, m int) string { return time.Date(2026, 9, d, h, m, 0, 0, time.Local).Format(time.RFC3339) }
	cases := []struct {
		name string
		job  ScheduledJob
		want string
	}{
		{"daily later today", ScheduledJob{ScheduleTime: "14:00"}, at(23, 14, 0)},
		{"daily passed today", ScheduledJob{TriggerMode: "daily", ScheduleTime: "08:00"}, at(24, 8, 0)},
		{"daily Friday only", ScheduledJob{ScheduleTime: "14:00", DaysOfWeek: []string{"Fri"}}, at(25, 14, 0)},
		{"daily Wednesday passed", ScheduledJob{ScheduleTime: "08:00", DaysOfWeek: []string{"Wed"}}, at(30, 8, 0)},
		{"every 120 min all day", ScheduledJob{TriggerMode: "interval", IntervalMinutes: 120, WindowAllDay: true}, at(23, 12, 0)},
		{"every 60 min 09-17", ScheduledJob{TriggerMode: "interval", IntervalMinutes: 60, WindowStart: "09:00", WindowEnd: "17:00"}, at(23, 11, 0)},
		{"every 5 min before window", ScheduledJob{TriggerMode: "interval", IntervalMinutes: 5, WindowStart: "14:00", WindowEnd: "18:00"}, at(23, 14, 0)},
		{"every 600 min past window", ScheduledJob{TriggerMode: "interval", IntervalMinutes: 600, WindowStart: "08:00", WindowEnd: "17:00"}, at(24, 8, 0)},
		{"every 120 min Friday only", ScheduledJob{TriggerMode: "interval", IntervalMinutes: 120, WindowAllDay: true, DaysOfWeek: []string{"Fri"}}, at(25, 0, 0)},
		{"zero interval", ScheduledJob{TriggerMode: "interval", WindowAllDay: true}, ""},
		{"manual", ScheduledJob{TriggerMode: "manual", ScheduleTime: "14:00"}, ""},
	}
	for _, c := range cases {
		if got := calculateNextRunAt(c.job, testNow); got != c.want {
			t.Errorf("%s: got %s, want %s", c.name, got, c.want)
		}
	}
}

func TestCheckUniqueJobName(t *testing.T) {
	jobs := []ScheduledJob{{ID: "1", Name: "Deepthought - Data"}, {ID: "2", Name: "Beeby Property"}}
	if err := checkUniqueJobName(jobs, ScheduledJob{ID: "3", Name: "  deepthought - DATA "}); err == nil {
		t.Error("duplicate name (other case, spaces) accepted")
	}
	if err := checkUniqueJobName(jobs, ScheduledJob{ID: "1", Name: "Deepthought - Data"}); err != nil {
		t.Errorf("saving a set under its own name refused: %v", err)
	}
	if err := checkUniqueJobName(jobs, ScheduledJob{ID: "3", Name: "Beeby Trading"}); err != nil {
		t.Errorf("new name refused: %v", err)
	}
}
