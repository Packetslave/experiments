package tui

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// parseWhen turns quick date shorthand into a concrete local time. Supported:
//
//	""                    → nil (clear the date)
//	today / tod           → today
//	tomorrow / tom        → tomorrow
//	mon..sun (or full)    → next occurrence of that weekday (always future)
//	3d / +3d / 2w         → N days/weeks from today
//	2026-08-20            → that date
//	2026-08-20 14:30      → that date and time
//	14:30                 → today at that time
//
// Forms without an explicit time use defaultHour (e.g. 8:00 for defer,
// 17:00 for due).
func parseWhen(input string, now time.Time, defaultHour int) (*time.Time, error) {
	s := strings.ToLower(strings.TrimSpace(input))
	if s == "" {
		return nil, nil
	}

	atHour := func(t time.Time) *time.Time {
		out := time.Date(t.Year(), t.Month(), t.Day(), defaultHour, 0, 0, 0, now.Location())
		return &out
	}

	switch s {
	case "today", "tod":
		return atHour(now), nil
	case "tomorrow", "tom":
		return atHour(now.AddDate(0, 0, 1)), nil
	}

	if wd, ok := weekdayFor(s); ok {
		days := (int(wd) - int(now.Weekday()) + 7) % 7
		if days == 0 {
			days = 7
		}
		return atHour(now.AddDate(0, 0, days)), nil
	}

	if m := relRe.FindStringSubmatch(s); m != nil {
		n, _ := strconv.Atoi(m[1])
		if m[2] == "w" {
			n *= 7
		}
		return atHour(now.AddDate(0, 0, n)), nil
	}

	if t, err := time.ParseInLocation("2006-01-02 15:04", s, now.Location()); err == nil {
		return &t, nil
	}
	if t, err := time.ParseInLocation("2006-01-02", s, now.Location()); err == nil {
		return atHour(t), nil
	}
	if t, err := time.ParseInLocation("15:04", s, now.Location()); err == nil {
		out := time.Date(now.Year(), now.Month(), now.Day(), t.Hour(), t.Minute(), 0, 0, now.Location())
		return &out, nil
	}

	return nil, fmt.Errorf("can't parse %q (try: tomorrow, fri, 3d, 2w, 2026-08-20, 14:30)", input)
}

var relRe = regexp.MustCompile(`^\+?(\d+)([dw])$`)

var weekdays = map[string]time.Weekday{
	"mon": time.Monday, "monday": time.Monday,
	"tue": time.Tuesday, "tues": time.Tuesday, "tuesday": time.Tuesday,
	"wed": time.Wednesday, "wednesday": time.Wednesday,
	"thu": time.Thursday, "thur": time.Thursday, "thurs": time.Thursday, "thursday": time.Thursday,
	"fri": time.Friday, "friday": time.Friday,
	"sat": time.Saturday, "saturday": time.Saturday,
	"sun": time.Sunday, "sunday": time.Sunday,
}

func weekdayFor(s string) (time.Weekday, bool) {
	wd, ok := weekdays[s]
	return wd, ok
}

// fmtWhen renders a date compactly for the task card.
func fmtWhen(t *time.Time) string {
	if t == nil {
		return ""
	}
	if t.Hour() == 0 && t.Minute() == 0 {
		return t.Format("Mon Jan 2 2006")
	}
	return t.Format("Mon Jan 2 2006 15:04")
}
