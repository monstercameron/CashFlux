// SPDX-License-Identifier: MIT

package smart

import "testing"

func TestDismissAll(t *testing.T) {
	s := Settings{}.DismissAll([]string{"a", "b", "c"})
	for _, k := range []string{"a", "b", "c"} {
		if !s.IsDismissed(k) {
			t.Errorf("key %q should be dismissed", k)
		}
	}
	if s.IsDismissed("d") {
		t.Error("key d was never dismissed")
	}
	// Empty input is a no-op, not a panic.
	if got := (Settings{}).DismissAll(nil); len(got.Dismissed) != 0 {
		t.Errorf("DismissAll(nil) dismissed %v, want none", got.Dismissed)
	}
}

func TestSnoozeUntilAndIsSnoozed(t *testing.T) {
	const now = int64(1000)
	// Not snoozed by default.
	if (Settings{}).IsSnoozed(now) {
		t.Error("zero value should not be snoozed")
	}
	// Snoozed while now is before the timestamp.
	s := Settings{}.SnoozeUntil(now + 100)
	if !s.IsSnoozed(now) {
		t.Error("should be snoozed before the deadline")
	}
	// Not snoozed once now reaches/passes it.
	if s.IsSnoozed(now + 100) {
		t.Error("should not be snoozed at the deadline")
	}
	if s.IsSnoozed(now + 200) {
		t.Error("should not be snoozed after the deadline")
	}
}

func TestClearGeneratedDropsSnooze(t *testing.T) {
	s := Settings{}.SnoozeUntil(9999).DismissAll([]string{"x"})
	got := s.ClearGenerated()
	if got.SnoozedUntil != 0 {
		t.Errorf("SnoozedUntil = %d, want 0 after ClearGenerated", got.SnoozedUntil)
	}
	if got.IsDismissed("x") {
		t.Error("dismissed keys should be cleared by ClearGenerated")
	}
}
