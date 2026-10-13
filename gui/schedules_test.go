package main

import (
	"strings"
	"testing"
	"time"
)

// nextAt returns s.next(after) formatted, or "" when there is none.
func nextAt(s Schedule, after time.Time) string {
	t, ok := s.next(after)
	if !ok {
		return ""
	}
	return t.Format("Mon 2006-01-02 15:04")
}

func TestScheduleNext(t *testing.T) {
	loc := time.Local
	at := func(y, mo, d, h, m int) time.Time { return time.Date(y, time.Month(mo), d, h, m, 0, 0, loc) }
	cases := []struct {
		name  string
		s     Schedule
		after time.Time
		want  string
	}{
		// Repeats run on the clock.
		{"every 2 hours from midnight", Schedule{Unit: "hours", Every: 2}, at(2026, 10, 14, 10, 1), "Wed 2026-10-14 12:00"},
		{"every hour at :15", Schedule{Unit: "hours", Every: 1, Minute: 15}, at(2026, 10, 14, 10, 15), "Wed 2026-10-14 11:15"},
		{"every 2 hours 08-18 weekdays, Friday evening", Schedule{Unit: "hours", Every: 2, WindowStart: "08:00", WindowEnd: "18:00", Weekdays: []string{"Mon", "Tue", "Wed", "Thu", "Fri"}}, at(2026, 10, 16, 18, 0), "Mon 2026-10-19 08:00"},
		{"every 30 minutes, window end included", Schedule{Unit: "minutes", Every: 30, WindowStart: "09:00", WindowEnd: "17:00"}, at(2026, 10, 14, 16, 31), "Wed 2026-10-14 17:00"},
		// Days.
		{"every day at 02:00", Schedule{Unit: "days", Every: 1, At: "02:00"}, at(2026, 10, 14, 2, 0), "Thu 2026-10-15 02:00"},
		{"every 3 days from the start date", Schedule{Unit: "days", Every: 3, At: "02:00", Start: "2026-10-01"}, at(2026, 10, 14, 3, 0), "Fri 2026-10-16 02:00"},
		{"start date in the future", Schedule{Unit: "days", Every: 1, At: "02:00", Start: "2026-11-01"}, at(2026, 10, 14, 3, 0), "Sun 2026-11-01 02:00"},
		// Weeks.
		{"every week Sat and Sun", Schedule{Unit: "weeks", Every: 1, At: "01:00", Weekdays: []string{"Sat", "Sun"}}, at(2026, 10, 14, 0, 0), "Sat 2026-10-17 01:00"},
		// Start Monday 5 October: the weeks of 5 and 19 October are on.
		{"every 2 weeks on Sat, skips the off week", Schedule{Unit: "weeks", Every: 2, At: "01:00", Weekdays: []string{"Sat"}, Start: "2026-10-05"}, at(2026, 10, 11, 0, 0), "Sat 2026-10-24 01:00"},
		{"every 2 weeks on Sat, after a run", Schedule{Unit: "weeks", Every: 2, At: "01:00", Weekdays: []string{"Sat"}, Start: "2026-10-05"}, at(2026, 10, 24, 2, 0), "Sat 2026-11-07 01:00"},
		// Months.
		{"last day of the month", Schedule{Unit: "months", Every: 1, At: "03:30", MonthMode: "last"}, at(2026, 10, 14, 0, 0), "Sat 2026-10-31 03:30"},
		{"last day, February", Schedule{Unit: "months", Every: 1, At: "03:30", MonthMode: "last"}, at(2027, 2, 1, 0, 0), "Sun 2027-02-28 03:30"},
		{"31st in a 30-day month: nearest earlier day", Schedule{Unit: "months", Every: 1, At: "03:00", MonthMode: "day", MonthDay: 31}, at(2026, 11, 1, 0, 0), "Mon 2026-11-30 03:00"},
		{"31st in February: nearest earlier day", Schedule{Unit: "months", Every: 1, At: "03:00", MonthMode: "day", MonthDay: 31}, at(2028, 2, 1, 0, 0), "Tue 2028-02-29 03:00"},
		{"2nd Tuesday", Schedule{Unit: "months", Every: 1, At: "22:00", MonthMode: "nth", Nth: 2, NthWeekday: "Tue"}, at(2026, 10, 14, 0, 0), "Tue 2026-11-10 22:00"},
		{"last Friday", Schedule{Unit: "months", Every: 1, At: "18:00", MonthMode: "nth", Nth: -1, NthWeekday: "Fri"}, at(2026, 10, 14, 0, 0), "Fri 2026-10-30 18:00"},
		{"1st Sunday", Schedule{Unit: "months", Every: 1, At: "03:00", MonthMode: "nth", Nth: 1, NthWeekday: "Sun"}, at(2026, 10, 14, 0, 0), "Sun 2026-11-01 03:00"},
		{"every 3 months on the 1st", Schedule{Unit: "months", Every: 3, At: "04:00", MonthMode: "day", MonthDay: 1, Start: "2026-01-01"}, at(2026, 10, 14, 0, 0), "Fri 2027-01-01 04:00"},
		// Years.
		{"every year on 31 December", Schedule{Unit: "years", Every: 1, At: "23:00", YearMonth: 12, YearDay: 31}, at(2026, 10, 14, 0, 0), "Thu 2026-12-31 23:00"},
		{"29 February in a common year: 28th", Schedule{Unit: "years", Every: 1, At: "12:00", YearMonth: 2, YearDay: 29}, at(2026, 10, 14, 0, 0), "Sun 2027-02-28 12:00"},
		{"29 February in a leap year", Schedule{Unit: "years", Every: 1, At: "12:00", YearMonth: 2, YearDay: 29}, at(2027, 3, 1, 0, 0), "Tue 2028-02-29 12:00"},
	}
	for _, c := range cases {
		if got := nextAt(c.s, c.after); got != c.want {
			t.Errorf("%s: got %q, want %q", c.name, got, c.want)
		}
	}
}

// Clock changes in the UK: on 28 March 2027 the clocks skip 01:00-02:00, and on
// 25 October 2026 01:00-02:00 happens twice.
func TestScheduleNextClockChanges(t *testing.T) {
	london, err := time.LoadLocation("Europe/London")
	if err != nil {
		t.Skipf("no time zone data: %v", err)
	}
	daily := Schedule{Unit: "days", Every: 1, At: "01:30"}

	// The skipped 01:30 runs once, an hour later by the clock (02:30 BST).
	got, _ := daily.next(time.Date(2027, 3, 28, 0, 0, 0, 0, london))
	if got.Hour() != 2 || got.Minute() != 30 || got.Day() != 28 {
		t.Errorf("spring: got %v, want 28 March 02:30", got)
	}

	// The doubled 01:30 runs at the first one only.
	first, _ := daily.next(time.Date(2026, 10, 25, 0, 0, 0, 0, london))
	if first.Day() != 25 || first.Hour() != 1 || first.Minute() != 30 {
		t.Fatalf("autumn: got %v, want 25 October 01:30", first)
	}
	second, _ := daily.next(first.Add(time.Minute))
	if second.Day() != 26 {
		t.Errorf("autumn: after the first 01:30 got %v, want 26 October", second)
	}

	// A day is a calendar day across the change, not 24 hours.
	got, _ = daily.next(time.Date(2026, 10, 24, 2, 0, 0, 0, london))
	if got.Day() != 25 || got.Hour() != 1 || got.Minute() != 30 {
		t.Errorf("across the change: got %v, want 25 October 01:30", got)
	}
}

// A set runs at its earliest enabled schedule; two due together give one time.
func TestNextRunForEarliestSchedule(t *testing.T) {
	now := time.Date(2026, 10, 14, 10, 0, 0, 0, time.Local)
	job := ScheduledJob{ScheduleMode: ScheduleModeScheduled, Schedules: []Schedule{
		{ID: "s1", Enabled: true, Unit: "days", Every: 1, At: "02:00"},
		{ID: "s2", Enabled: true, Unit: "months", Every: 1, At: "03:30", MonthMode: "last"},
		{ID: "s3", Enabled: false, Unit: "hours", Every: 1},
	}}
	next, by := nextRunFor(job, now)
	if want := time.Date(2026, 10, 15, 2, 0, 0, 0, time.Local).Format(time.RFC3339); next != want || by != "s1" {
		t.Errorf("got %s by %s, want %s by s1 (the hourly one is off)", next, by, want)
	}

	job.Schedules[1].At = "02:00"
	job.Schedules[1].MonthMode, job.Schedules[1].MonthDay = "day", 15
	next, by = nextRunFor(job, now)
	if want := time.Date(2026, 10, 15, 2, 0, 0, 0, time.Local).Format(time.RFC3339); next != want || by != "s1" {
		t.Errorf("due together: got %s by %s, want %s once", next, by, want)
	}

	job.ScheduleMode = ScheduleModeManual
	if next, by = nextRunFor(job, now); next != "" || by != "" {
		t.Errorf("manual set got a next run %q", next)
	}
	job.ScheduleMode = ScheduleModeScheduled
	for i := range job.Schedules {
		job.Schedules[i].Enabled = false
	}
	if next, _ = nextRunFor(job, now); next != "" {
		t.Errorf("all schedules off, got a next run %q", next)
	}
}

// Old sets move over to one schedule; the old fields are then written back
// from it where they can say it, and "manual" where they cannot.
func TestNormalizeJobSchedules(t *testing.T) {
	job := ScheduledJob{TriggerMode: "interval", IntervalMinutes: 90, WindowStart: "09:00", WindowEnd: "17:00", DaysOfWeek: []string{"Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun"}}
	normalizeJobSchedules(&job)
	if job.ScheduleMode != ScheduleModeScheduled || len(job.Schedules) != 1 {
		t.Fatalf("got %+v", job)
	}
	s := job.Schedules[0]
	if s.ID != "s1" || s.Unit != "minutes" || s.Every != 90 || s.WindowStart != "09:00" || s.Weekdays != nil {
		t.Errorf("interval 90 moved over as %+v", s)
	}
	if job.TriggerMode != "interval" || job.IntervalMinutes != 90 || job.WindowAllDay {
		t.Errorf("mirror: %+v", job)
	}

	job = ScheduledJob{TriggerMode: "interval", IntervalMinutes: 120, WindowAllDay: true}
	normalizeJobSchedules(&job)
	if s := job.Schedules[0]; s.Unit != "hours" || s.Every != 2 {
		t.Errorf("interval 120 should become every 2 hours, got %+v", s)
	}

	// A monthly first schedule cannot be said the old way: older readers see manual.
	job = ScheduledJob{ScheduleMode: ScheduleModeScheduled, TriggerMode: "daily", ScheduleTime: "02:00", Schedules: []Schedule{
		{Enabled: true, Unit: "months", Every: 1, At: "03:00", MonthMode: "last"},
		{ID: "s1", Enabled: true, Unit: "days", Every: 1, At: "02:00"},
	}}
	normalizeJobSchedules(&job)
	if job.Schedules[0].ID != "s2" {
		t.Errorf("new schedule got ID %q, want s2 (s1 is taken)", job.Schedules[0].ID)
	}
	if job.TriggerMode != "manual" {
		t.Errorf("monthly first: older readers should see manual, got %q", job.TriggerMode)
	}

	// Every week on chosen days is the old daily with days.
	job = ScheduledJob{ScheduleMode: ScheduleModeScheduled, Schedules: []Schedule{{Enabled: true, Unit: "weeks", Every: 1, At: "01:00", Weekdays: []string{"Sat"}}}}
	normalizeJobSchedules(&job)
	if job.TriggerMode != "daily" || job.ScheduleTime != "01:00" || len(job.DaysOfWeek) != 1 {
		t.Errorf("weekly mirror: %+v", job)
	}
}

func TestValidateSchedules(t *testing.T) {
	ok := []Schedule{
		{Unit: "minutes", Every: 5, WindowStart: "09:00", WindowEnd: "17:00"},
		{Unit: "hours", Every: 1, Minute: 30},
		{Unit: "days", Every: 2, At: "02:00", Start: "2026-10-14"},
		{Unit: "weeks", Every: 1, At: "01:00", Weekdays: []string{"Sat"}},
		{Unit: "months", Every: 1, At: "03:00", MonthMode: "day", MonthDay: 31},
		{Unit: "months", Every: 1, At: "03:00", MonthMode: "nth", Nth: -1, NthWeekday: "Fri"},
		{Unit: "years", Every: 1, At: "12:00", YearMonth: 2, YearDay: 29},
	}
	for _, s := range ok {
		if err := validateSchedules(ScheduledJob{ScheduleMode: ScheduleModeScheduled, Schedules: []Schedule{s}}); err != nil {
			t.Errorf("%+v: %v", s, err)
		}
	}
	bad := map[string]Schedule{
		"shortest repeat":   {Unit: "minutes", Every: 4},
		"unknown unit":      {Unit: "fortnights", Every: 1, At: "02:00"},
		"window backwards":  {Unit: "hours", Every: 1, WindowStart: "17:00", WindowEnd: "09:00"},
		"half a window":     {Unit: "hours", Every: 1, WindowStart: "09:00"},
		"time":              {Unit: "days", Every: 1, At: "25:00"},
		"no day of week":    {Unit: "weeks", Every: 1, At: "01:00"},
		"day 32":            {Unit: "months", Every: 1, At: "03:00", MonthMode: "day", MonthDay: 32},
		"5th weekday":       {Unit: "months", Every: 1, At: "03:00", MonthMode: "nth", Nth: 5, NthWeekday: "Fri"},
		"30 February":       {Unit: "years", Every: 1, At: "12:00", YearMonth: 2, YearDay: 30},
		"start date format": {Unit: "days", Every: 1, At: "02:00", Start: "14/10/2026"},
		"every 0":           {Unit: "days", Every: 0, At: "02:00"},
	}
	for name, s := range bad {
		if err := validateSchedules(ScheduledJob{ScheduleMode: ScheduleModeScheduled, Schedules: []Schedule{s}}); err == nil || !strings.HasPrefix(err.Error(), "schedule 1:") {
			t.Errorf("%s: got %v, want a schedule 1 error", name, err)
		}
	}
}
