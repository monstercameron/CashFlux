// SPDX-License-Identifier: MIT

package notify

import (
	"testing"
	"time"
)

// at builds a local time at the given clock time on a fixed date.
func at(hour, min int) time.Time {
	return time.Date(2026, time.June, 18, hour, min, 0, 0, time.UTC)
}

func TestRuleInQuietHours(t *testing.T) {
	cases := []struct {
		name       string
		start, end int
		clock      time.Time
		want       bool
	}{
		{"disabled when equal", 0, 0, at(3, 0), false},
		{"daytime window inside", 9 * 60, 17 * 60, at(12, 0), true},
		{"daytime window before", 9 * 60, 17 * 60, at(8, 59), false},
		{"daytime window start inclusive", 9 * 60, 17 * 60, at(9, 0), true},
		{"daytime window end exclusive", 9 * 60, 17 * 60, at(17, 0), false},
		{"overnight wrap late evening", 22 * 60, 7 * 60, at(23, 0), true},
		{"overnight wrap early morning", 22 * 60, 7 * 60, at(6, 0), true},
		{"overnight wrap midday outside", 22 * 60, 7 * 60, at(12, 0), false},
		{"overnight wrap end exclusive", 22 * 60, 7 * 60, at(7, 0), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := Rule{QuietStartMin: tc.start, QuietEndMin: tc.end}
			if got := r.InQuietHours(tc.clock); got != tc.want {
				t.Errorf("InQuietHours(%s) = %v, want %v", tc.clock.Format("15:04"), got, tc.want)
			}
		})
	}
}

func TestRuleHasChannel(t *testing.T) {
	r := Rule{Channels: []Channel{ChannelInApp}}
	if !r.HasChannel(ChannelInApp) {
		t.Error("expected in-app channel present")
	}
	if r.HasChannel(ChannelBrowser) {
		t.Error("did not expect browser channel")
	}
}

func TestRuleCanFireAt(t *testing.T) {
	base := Rule{Enabled: true, Channels: []Channel{ChannelInApp}}
	if !base.CanFireAt(at(12, 0)) {
		t.Error("enabled rule with a channel and no quiet hours should be able to fire")
	}
	if (Rule{Enabled: false, Channels: []Channel{ChannelInApp}}).CanFireAt(at(12, 0)) {
		t.Error("disabled rule must not fire")
	}
	if (Rule{Enabled: true}).CanFireAt(at(12, 0)) {
		t.Error("rule with no channels must not fire")
	}
	quiet := Rule{Enabled: true, Channels: []Channel{ChannelInApp}, QuietStartMin: 22 * 60, QuietEndMin: 7 * 60}
	if quiet.CanFireAt(at(23, 0)) {
		t.Error("rule inside quiet hours must not fire")
	}
	if !quiet.CanFireAt(at(12, 0)) {
		t.Error("rule outside quiet hours should fire")
	}
}

func TestDedupeKeyDeterministic(t *testing.T) {
	a := DedupeKey("rule-1", "2026-06")
	b := DedupeKey("rule-1", "2026-06")
	if a != b {
		t.Errorf("DedupeKey not deterministic: %q vs %q", a, b)
	}
	if DedupeKey("rule-1", "2026-06") == DedupeKey("rule-1", "2026-07") {
		t.Error("different occurrences must produce different keys")
	}
	if DedupeKey("rule-1", "x") == DedupeKey("rule-2", "x") {
		t.Error("different rules must produce different keys")
	}
}

func TestPeriodKeys(t *testing.T) {
	d := time.Date(2026, time.June, 18, 9, 30, 0, 0, time.UTC) // a Thursday
	if got := DayKey(d); got != "2026-06-18" {
		t.Errorf("DayKey = %q, want 2026-06-18", got)
	}
	if got := MonthKey(d); got != "2026-06" {
		t.Errorf("MonthKey = %q, want 2026-06", got)
	}
	if got := WeekKey(d); got != "2026-W25" {
		t.Errorf("WeekKey = %q, want 2026-W25", got)
	}
}

func TestDeliveredLog(t *testing.T) {
	l := NewDeliveredLog()
	key := DedupeKey("r", "2026-06")
	if l.Has(key) {
		t.Error("fresh log should not have the key")
	}
	l.Mark(key)
	if !l.Has(key) {
		t.Error("marked key should be present")
	}
	l.Mark(DedupeKey("r", "2026-05"))
	want := []string{"r@2026-05", "r@2026-06"}
	got := l.Keys()
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Errorf("Keys() = %v, want %v (sorted)", got, want)
	}
	// Has must be safe on a nil log.
	var nilLog DeliveredLog
	if nilLog.Has("anything") {
		t.Error("nil log should report no keys")
	}
}
