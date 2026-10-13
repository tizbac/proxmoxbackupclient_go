package main

import (
	"fmt"
	"strings"
	"time"
)

// Schedule is one of a Backup Set's schedules: "every N <unit>" plus only the
// fields that unit needs. A set runs at the earliest time any of its enabled
// schedules gives (ScheduledJob.NextRun); two due at the same minute give one
// run. Times are wall-clock times in the computer's own time zone.
type Schedule struct {
	ID      string `json:"id"`
	Enabled bool   `json:"enabled"`
	// Unit is "minutes", "hours", "days", "weeks", "months" or "years".
	Unit  string `json:"unit"`
	Every int    `json:"every"` // N, at least 1
	// At is the "HH:MM" of a days, weeks, months or years schedule.
	At string `json:"at,omitempty"`
	// Minute is the minute past the hour of an all-day hours schedule.
	Minute int `json:"minute,omitempty"`
	// WindowStart/WindowEnd ("HH:MM") limit a minutes or hours schedule to
	// part of the day; both empty means all day. The repeats run on the clock
	// from WindowStart (or midnight): every 2 hours from 09:00 is 09:00, 11:00...
	WindowStart string `json:"windowStart,omitempty"`
	WindowEnd   string `json:"windowEnd,omitempty"`
	// Weekdays ("Mon".."Sun"): the days of a weeks schedule; for minutes,
	// hours and days schedules, the only days they run on (empty = every day).
	Weekdays []string `json:"weekdays,omitempty"`
	// Months: MonthMode "day" (MonthDay, or the nearest earlier day a short
	// month has), "last" (the last day) or "nth" (the Nth NthWeekday; Nth -1
	// is the last one).
	MonthMode  string `json:"monthMode,omitempty"`
	MonthDay   int    `json:"monthDay,omitempty"`
	Nth        int    `json:"nth,omitempty"`
	NthWeekday string `json:"nthWeekday,omitempty"`
	// Years: YearMonth (1-12) and YearDay; 29 February is 28 February in
	// other years (the nearest earlier day).
	YearMonth int `json:"yearMonth,omitempty"`
	YearDay   int `json:"yearDay,omitempty"`
	// Start ("YYYY-MM-DD") is the first day the schedule may run, and what
	// "every 2 days/weeks/months/years" counts from. Empty: no start, and the
	// count runs from Monday 3 January 2000.
	Start string `json:"start,omitempty"`
}

// ScheduleMode values for ScheduledJob.ScheduleMode.
const (
	ScheduleModeScheduled = "scheduled"
	ScheduleModeManual    = "manual"
)

var scheduleUnits = map[string]bool{"minutes": true, "hours": true, "days": true, "weeks": true, "months": true, "years": true}

// minRepeatMinutes is the shortest "every N minutes" a set may be saved with.
const minRepeatMinutes = 5

// jobSchedules returns a set's schedule mode and schedules. A set saved before
// schedules existed (ScheduleMode "") is read from its old trigger fields:
// "daily" (or "") at ScheduleTime on DaysOfWeek, "interval" every
// IntervalMinutes in the window, "manual".
func jobSchedules(job ScheduledJob) (string, []Schedule) {
	if job.ScheduleMode != "" {
		return job.ScheduleMode, job.Schedules
	}
	switch job.TriggerMode {
	case "manual":
		return ScheduleModeManual, nil
	case "interval":
		if job.IntervalMinutes <= 0 {
			return ScheduleModeScheduled, nil
		}
		s := Schedule{ID: "s1", Enabled: true, Unit: "minutes", Every: job.IntervalMinutes, Weekdays: everyDayAsNone(job.DaysOfWeek)}
		if job.IntervalMinutes%60 == 0 {
			s.Unit, s.Every = "hours", job.IntervalMinutes/60
		}
		if !job.WindowAllDay {
			s.WindowStart, s.WindowEnd = job.WindowStart, job.WindowEnd
		}
		return ScheduleModeScheduled, []Schedule{s}
	default:
		if job.ScheduleTime == "" {
			return ScheduleModeScheduled, nil
		}
		return ScheduleModeScheduled, []Schedule{{ID: "s1", Enabled: true, Unit: "days", Every: 1, At: job.ScheduleTime, Weekdays: everyDayAsNone(job.DaysOfWeek)}}
	}
}

// everyDayAsNone returns nil for "all seven days" (the same as none).
func everyDayAsNone(days []string) []string {
	if len(days) >= 7 {
		return nil
	}
	return days
}

// normalizeJobSchedules moves a set to the schedule model (ScheduleMode set,
// Schedules filled, every schedule given an ID) and writes the old trigger
// fields from it, so an older app or service reading the file runs the set
// at the right times or, where the old model cannot say it, not at all.
func normalizeJobSchedules(job *ScheduledJob) {
	job.ScheduleMode, job.Schedules = jobSchedules(*job)
	used := map[string]bool{}
	for _, s := range job.Schedules {
		used[s.ID] = true
	}
	for i := range job.Schedules {
		if job.Schedules[i].ID != "" {
			continue
		}
		for n := 1; ; n++ {
			id := fmt.Sprintf("s%d", n)
			if !used[id] {
				job.Schedules[i].ID, used[id] = id, true
				break
			}
		}
	}
	mirrorLegacyTrigger(job)
}

// mirrorLegacyTrigger writes TriggerMode and its fields from the set's first
// enabled schedule when the old model can express it (every day, every week,
// or a repeat in minutes or hours), and "manual" otherwise.
func mirrorLegacyTrigger(job *ScheduledJob) {
	job.TriggerMode = "manual"
	if job.ScheduleMode == ScheduleModeManual {
		return
	}
	for _, s := range job.Schedules {
		if !s.Enabled {
			continue
		}
		switch {
		case s.Unit == "days" && s.Every == 1, s.Unit == "weeks" && s.Every == 1:
			job.TriggerMode, job.ScheduleTime, job.DaysOfWeek = "daily", s.At, s.Weekdays
		case s.Unit == "minutes" || s.Unit == "hours":
			mins := s.Every
			if s.Unit == "hours" {
				mins *= 60
			}
			job.TriggerMode, job.IntervalMinutes, job.DaysOfWeek = "interval", mins, s.Weekdays
			job.WindowAllDay = s.WindowStart == "" || s.WindowEnd == ""
			job.WindowStart, job.WindowEnd = s.WindowStart, s.WindowEnd
		}
		return
	}
}

// validateSchedules checks a set's schedules before it is saved.
func validateSchedules(job ScheduledJob) error {
	mode, list := jobSchedules(job)
	if mode != ScheduleModeScheduled && mode != ScheduleModeManual {
		return fmt.Errorf("unknown schedule mode %q", mode)
	}
	for i, s := range list {
		n := i + 1
		if !scheduleUnits[s.Unit] {
			return fmt.Errorf("schedule %d: unknown unit %q", n, s.Unit)
		}
		if s.Every < 1 {
			return fmt.Errorf("schedule %d: repeat every must be at least 1", n)
		}
		if s.Unit == "minutes" && s.Every < minRepeatMinutes {
			return fmt.Errorf("schedule %d: the shortest repeat is %d minutes", n, minRepeatMinutes)
		}
		for _, d := range s.Weekdays {
			if weekdayIndex(d) < 0 {
				return fmt.Errorf("schedule %d: unknown day %q", n, d)
			}
		}
		if s.Start != "" {
			if _, err := time.Parse("2006-01-02", s.Start); err != nil {
				return fmt.Errorf("schedule %d: start date %q is not YYYY-MM-DD", n, s.Start)
			}
		}
		switch s.Unit {
		case "minutes", "hours":
			if (s.WindowStart == "") != (s.WindowEnd == "") {
				return fmt.Errorf("schedule %d: a time window needs a start and an end", n)
			}
			if s.WindowStart != "" {
				a, err1 := hhmmMinutes(s.WindowStart)
				b, err2 := hhmmMinutes(s.WindowEnd)
				if err1 != nil || err2 != nil {
					return fmt.Errorf("schedule %d: window times must be HH:MM", n)
				}
				if b <= a {
					return fmt.Errorf("schedule %d: the window must end after it starts", n)
				}
			}
			if s.Minute < 0 || s.Minute > 59 {
				return fmt.Errorf("schedule %d: minute past the hour must be 0 to 59", n)
			}
			continue
		}
		if _, err := hhmmMinutes(s.At); err != nil {
			return fmt.Errorf("schedule %d: time %q is not HH:MM", n, s.At)
		}
		switch s.Unit {
		case "weeks":
			if len(s.Weekdays) == 0 {
				return fmt.Errorf("schedule %d: pick at least one day of the week", n)
			}
		case "months":
			switch s.MonthMode {
			case "day":
				if s.MonthDay < 1 || s.MonthDay > 31 {
					return fmt.Errorf("schedule %d: day of the month must be 1 to 31", n)
				}
			case "last":
			case "nth":
				if (s.Nth < 1 || s.Nth > 4) && s.Nth != -1 {
					return fmt.Errorf("schedule %d: which weekday of the month must be 1st to 4th or last", n)
				}
				if weekdayIndex(s.NthWeekday) < 0 {
					return fmt.Errorf("schedule %d: unknown day %q", n, s.NthWeekday)
				}
			default:
				return fmt.Errorf("schedule %d: unknown day of the month %q", n, s.MonthMode)
			}
		case "years":
			if s.YearMonth < 1 || s.YearMonth > 12 {
				return fmt.Errorf("schedule %d: month must be 1 to 12", n)
			}
			if s.YearDay < 1 || s.YearDay > daysIn(2000, time.Month(s.YearMonth)) { // 2000: a leap year, so 29 February is allowed
				return fmt.Errorf("schedule %d: %s has no day %d", n, time.Month(s.YearMonth), s.YearDay)
			}
		}
	}
	return nil
}

// nextRunFor returns a set's next run (RFC 3339) after now and the ID of the
// schedule that gives it; both empty for a manual set or one with no enabled
// schedule.
func nextRunFor(job ScheduledJob, now time.Time) (string, string) {
	mode, list := jobSchedules(job)
	if mode == ScheduleModeManual {
		return "", ""
	}
	var best time.Time
	by := ""
	for _, s := range list {
		if !s.Enabled {
			continue
		}
		if t, ok := s.next(now); ok && (best.IsZero() || t.Before(best)) {
			best, by = t, s.ID
		}
	}
	if best.IsZero() {
		return "", ""
	}
	return best.Format(time.RFC3339), by
}

// setNextRun fills a set's NextRun and NextRunBy from now.
func setNextRun(job *ScheduledJob, now time.Time) {
	job.NextRun, job.NextRunBy = nextRunFor(*job, now)
}

// next returns the schedule's first run strictly after `after`. It walks
// forward a day at a time, so clock changes are handled as wall-clock times:
// a time the clocks skip runs at the time Go normalises it to (02:30 on the
// spring change is 03:30), and a time that happens twice runs at the first.
func (s Schedule) next(after time.Time) (time.Time, bool) {
	loc := after.Location()
	anchor := s.anchor(loc)
	every := s.Every
	if every < 1 {
		every = 1
	}
	day := time.Date(after.Year(), after.Month(), after.Day(), 0, 0, 0, 0, loc)
	if day.Before(anchor) {
		day = anchor
	}
	horizon := 8
	switch s.Unit {
	case "days":
		horizon = every + 8
	case "weeks":
		horizon = 7*every + 8
	case "months":
		horizon = 31*every + 62
	case "years":
		horizon = 366*every + 366*4 // 29 February may be four years off
	}
	for i := 0; i <= horizon; i++ {
		d := time.Date(day.Year(), day.Month(), day.Day()+i, 0, 0, 0, 0, loc)
		if !s.onDay(d, anchor, every) {
			continue
		}
		for _, t := range s.timesOn(d) {
			if t.After(after) {
				return t, true
			}
		}
	}
	return time.Time{}, false
}

// anchor is the schedule's start day (midnight in loc).
func (s Schedule) anchor(loc *time.Location) time.Time {
	if t, err := time.ParseInLocation("2006-01-02", s.Start, loc); err == nil {
		return t
	}
	return time.Date(2000, 1, 3, 0, 0, 0, 0, loc)
}

// onDay reports whether the schedule runs on day d (midnight).
func (s Schedule) onDay(d, anchor time.Time, every int) bool {
	switch s.Unit {
	case "minutes", "hours":
		return weekdayListed(d, s.Weekdays, true)
	case "days":
		return (civilDay(d)-civilDay(anchor))%every == 0 && weekdayListed(d, s.Weekdays, true)
	case "weeks":
		days := s.Weekdays
		if len(days) == 0 {
			days = []string{weekdayAbbrev[anchor.Weekday()]}
		}
		if !weekdayListed(d, days, false) {
			return false
		}
		return ((civilDay(mondayOf(d))-civilDay(mondayOf(anchor)))/7)%every == 0
	case "months":
		months := (d.Year()-anchor.Year())*12 + int(d.Month()) - int(anchor.Month())
		if months%every != 0 {
			return false
		}
		last := daysIn(d.Year(), d.Month())
		switch s.MonthMode {
		case "day":
			return d.Day() == min(s.MonthDay, last)
		case "last":
			return d.Day() == last
		case "nth":
			if weekdayAbbrev[d.Weekday()] != s.NthWeekday {
				return false
			}
			if s.Nth == -1 {
				return d.Day()+7 > last
			}
			return (d.Day()-1)/7+1 == s.Nth
		}
		return false
	case "years":
		if (d.Year()-anchor.Year())%every != 0 || int(d.Month()) != s.YearMonth {
			return false
		}
		return d.Day() == min(s.YearDay, daysIn(d.Year(), d.Month()))
	}
	return false
}

// timesOn lists the schedule's times on day d, in order.
func (s Schedule) timesOn(d time.Time) []time.Time {
	at := func(m int) time.Time {
		return time.Date(d.Year(), d.Month(), d.Day(), m/60, m%60, 0, 0, d.Location())
	}
	if s.Unit != "minutes" && s.Unit != "hours" {
		m, err := hhmmMinutes(s.At)
		if err != nil {
			return nil
		}
		return []time.Time{at(m)}
	}
	step := max(s.Every, 1)
	if s.Unit == "hours" {
		step *= 60
	}
	from, to := s.Minute, 24*60-1
	if s.WindowStart != "" && s.WindowEnd != "" {
		a, err1 := hhmmMinutes(s.WindowStart)
		b, err2 := hhmmMinutes(s.WindowEnd)
		if err1 != nil || err2 != nil {
			return nil
		}
		from, to = a, max(a, b)
	} else if s.Unit == "minutes" {
		from = 0
	}
	var out []time.Time
	for m := from; m <= to; m += step {
		out = append(out, at(m))
	}
	return out
}

// hhmmMinutes parses "HH:MM" into minutes after midnight.
func hhmmMinutes(s string) (int, error) {
	h, m, err := parseHHMM(s)
	if err != nil || len(s) < 3 || h < 0 || h > 23 || m < 0 || m > 59 {
		return 0, fmt.Errorf("not HH:MM: %q", s)
	}
	return h*60 + m, nil
}

// civilDay numbers calendar days (time zone and clock changes play no part).
func civilDay(t time.Time) int {
	y, m, d := t.Date()
	return int(time.Date(y, m, d, 0, 0, 0, 0, time.UTC).Unix() / 86400)
}

// mondayOf is the Monday of t's week.
func mondayOf(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day()-(int(t.Weekday())+6)%7, 0, 0, 0, 0, t.Location())
}

// daysIn is the number of days in month m of year y.
func daysIn(y int, m time.Month) int {
	return time.Date(y, m+1, 0, 0, 0, 0, 0, time.UTC).Day()
}

// weekdayIndex returns the index of "Sun".."Sat" in weekdayAbbrev, or -1.
func weekdayIndex(d string) int {
	for i, w := range weekdayAbbrev {
		if strings.EqualFold(w, d) {
			return i
		}
	}
	return -1
}

// weekdayListed reports whether d's weekday is in days; an empty list counts
// as every day when emptyIsAll.
func weekdayListed(d time.Time, days []string, emptyIsAll bool) bool {
	if len(days) == 0 {
		return emptyIsAll
	}
	return dayAllowed(d, days)
}
