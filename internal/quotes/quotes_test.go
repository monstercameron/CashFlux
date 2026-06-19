package quotes

import (
	"testing"
	"time"
)

func TestOfDayDeterministicAndRotates(t *testing.T) {
	d1 := time.Date(2026, time.June, 10, 9, 0, 0, 0, time.UTC)
	// Same calendar day (different time) → same quote.
	if OfDay(d1) != OfDay(d1.Add(8*time.Hour)) {
		t.Error("same day should return the same quote")
	}
	// Next day → advances by one in the set.
	next := OfDay(d1.AddDate(0, 0, 1))
	if next == OfDay(d1) && Count() > 1 {
		t.Error("consecutive days should rotate to a different quote")
	}
	// Full cycle returns to the same quote.
	if OfDay(d1) != OfDay(d1.AddDate(0, 0, Count())) {
		t.Error("a full cycle of days should return the original quote")
	}
}

func TestOfDayAlwaysValid(t *testing.T) {
	// Pre-epoch date (negative day index) must still return a real quote, not panic.
	old := time.Date(1969, time.January, 1, 0, 0, 0, 0, time.UTC)
	q := OfDay(old)
	if q.Text == "" {
		t.Error("pre-epoch date should still yield a quote")
	}
}

func TestAllIsCopy(t *testing.T) {
	a := All()
	if len(a) != Count() {
		t.Fatalf("All len %d != Count %d", len(a), Count())
	}
	a[0].Text = "mutated"
	if All()[0].Text == "mutated" {
		t.Error("All() must return a defensive copy")
	}
}
