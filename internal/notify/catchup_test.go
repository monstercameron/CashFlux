// SPDX-License-Identifier: MIT

package notify

import (
	"testing"
	"time"
)

func day(n int) time.Time {
	return time.Date(2026, time.June, n, 9, 0, 0, 0, time.UTC)
}

// enabledRule is a minimal eligible rule (enabled, one channel, no cap).
func enabledRule(id string, cap int) Rule {
	return Rule{ID: id, Event: EventBillDue, Enabled: true, Channels: []Channel{ChannelInApp}, FrequencyCap: cap}
}

func cand(ruleID string, n int) Candidate {
	return Candidate{RuleID: ruleID, Event: EventBillDue, OccurrenceKey: DayKey(day(n)), At: day(n), Title: "t", Body: "b"}
}

func TestCatchUpBasicAndNewestFirst(t *testing.T) {
	rules := []Rule{enabledRule("r1", 0)}
	cands := []Candidate{cand("r1", 10), cand("r1", 12), cand("r1", 11)}
	log := NewDeliveredLog()
	got := CatchUp(rules, cands, day(13), log)
	if len(got) != 3 {
		t.Fatalf("got %d notifications, want 3", len(got))
	}
	// Newest first.
	if !got[0].At.Equal(day(12)) || !got[2].At.Equal(day(10)) {
		t.Errorf("order = %s,%s,%s, want 12,11,10",
			got[0].At.Format("01-02"), got[1].At.Format("01-02"), got[2].At.Format("01-02"))
	}
	// All marked delivered.
	if len(log.Keys()) != 3 {
		t.Errorf("delivered log has %d keys, want 3", len(log.Keys()))
	}
}

func TestCatchUpGating(t *testing.T) {
	rules := []Rule{
		{ID: "disabled", Enabled: false, Channels: []Channel{ChannelInApp}},
		{ID: "nochan", Enabled: true},
		enabledRule("ok", 0),
	}
	cands := []Candidate{
		cand("disabled", 10),
		cand("nochan", 10),
		cand("unknown", 10),
		cand("ok", 10),
	}
	got := CatchUp(rules, cands, day(11), NewDeliveredLog())
	if len(got) != 1 || got[0].RuleID != "ok" {
		t.Fatalf("got %+v, want exactly the 'ok' rule's notification", got)
	}
}

func TestCatchUpIdempotent(t *testing.T) {
	rules := []Rule{enabledRule("r1", 0)}
	cands := []Candidate{cand("r1", 10), cand("r1", 11)}
	log := NewDeliveredLog()
	if first := CatchUp(rules, cands, day(12), log); len(first) != 2 {
		t.Fatalf("first run got %d, want 2", len(first))
	}
	if second := CatchUp(rules, cands, day(12), log); len(second) != 0 {
		t.Errorf("second run got %d, want 0 (already delivered)", len(second))
	}
}

func TestCatchUpFrequencyCapCollapses(t *testing.T) {
	rules := []Rule{enabledRule("r1", 2)} // cap 2
	cands := []Candidate{cand("r1", 10), cand("r1", 11), cand("r1", 12), cand("r1", 13), cand("r1", 14)}
	log := NewDeliveredLog()
	got := CatchUp(rules, cands, day(15), log)
	if len(got) != 2 {
		t.Fatalf("got %d, want 2 (capped)", len(got))
	}
	// Keeps the two most recent.
	if !got[0].At.Equal(day(14)) || !got[1].At.Equal(day(13)) {
		t.Errorf("kept %s,%s, want 14,13", got[0].At.Format("01-02"), got[1].At.Format("01-02"))
	}
	// All five marked delivered (so the dropped ones never replay).
	if len(log.Keys()) != 5 {
		t.Errorf("delivered log has %d keys, want 5", len(log.Keys()))
	}
	// And a re-run emits nothing.
	if again := CatchUp(rules, cands, day(15), log); len(again) != 0 {
		t.Errorf("re-run got %d, want 0", len(again))
	}
}

func TestCatchUpNilLogSafe(t *testing.T) {
	rules := []Rule{enabledRule("r1", 0)}
	got := CatchUp(rules, []Candidate{cand("r1", 10)}, day(11), nil)
	if len(got) != 1 {
		t.Errorf("got %d with nil log, want 1", len(got))
	}
}
