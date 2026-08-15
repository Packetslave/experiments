package tui

import (
	"testing"
	"time"
)

func TestParseWhen(t *testing.T) {
	// A fixed "now": Saturday 2026-08-15 10:30 local time.
	now := time.Date(2026, 8, 15, 10, 30, 0, 0, time.Local)

	cases := []struct {
		in   string
		hour int
		want *time.Time
	}{
		{"", 17, nil},
		{"   ", 17, nil},
		{"today", 17, timePtr(2026, 8, 15, 17, 0)},
		{"tod", 8, timePtr(2026, 8, 15, 8, 0)},
		{"tomorrow", 17, timePtr(2026, 8, 16, 17, 0)},
		{"tom", 8, timePtr(2026, 8, 16, 8, 0)},
		{"mon", 17, timePtr(2026, 8, 17, 17, 0)},
		{"Friday", 17, timePtr(2026, 8, 21, 17, 0)},
		// Same weekday as "now" means next week, not today.
		{"sat", 17, timePtr(2026, 8, 22, 17, 0)},
		{"3d", 8, timePtr(2026, 8, 18, 8, 0)},
		{"+3d", 8, timePtr(2026, 8, 18, 8, 0)},
		{"2w", 8, timePtr(2026, 8, 29, 8, 0)},
		{"2026-09-01", 17, timePtr(2026, 9, 1, 17, 0)},
		{"2026-09-01 14:45", 17, timePtr(2026, 9, 1, 14, 45)},
		{"14:45", 17, timePtr(2026, 8, 15, 14, 45)},
	}
	for _, c := range cases {
		got, err := parseWhen(c.in, now, c.hour)
		if err != nil {
			t.Errorf("parseWhen(%q): unexpected error: %v", c.in, err)
			continue
		}
		switch {
		case c.want == nil && got != nil:
			t.Errorf("parseWhen(%q) = %v, want nil", c.in, got)
		case c.want != nil && got == nil:
			t.Errorf("parseWhen(%q) = nil, want %v", c.in, *c.want)
		case c.want != nil && !got.Equal(*c.want):
			t.Errorf("parseWhen(%q) = %v, want %v", c.in, *got, *c.want)
		}
	}
}

func TestParseWhenErrors(t *testing.T) {
	now := time.Now()
	for _, in := range []string{"nonsense", "13-01", "0x", "next year"} {
		if _, err := parseWhen(in, now, 17); err == nil {
			t.Errorf("parseWhen(%q): expected error, got none", in)
		}
	}
}

func timePtr(y int, mo time.Month, d, h, min int) *time.Time {
	t := time.Date(y, mo, d, h, min, 0, 0, time.Local)
	return &t
}
