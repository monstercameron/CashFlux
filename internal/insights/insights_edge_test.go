package insights

import "testing"

// TestDetectDownAndSort covers the down-anomaly path (negative delta, Direction
// Down), the below-threshold drop that is skipped, and the sort comparator's
// abs64 on a negative delta (which only runs with two or more anomalies).
func TestDetectDownAndSort(t *testing.T) {
	series := []CategorySeries{
		{Category: "Travel", Spend: []int64{2000, 2000, 10000}},  // +400% → Up, flagged
		{Category: "Dining", Spend: []int64{10000, 10000, 2000}}, // -80%  → Down, flagged
		{Category: "Gas", Spend: []int64{10000, 10000, 9000}},    // -10%  → below 50% threshold, skipped
	}
	got := Detect(series, DefaultOptions())
	if len(got) != 2 {
		t.Fatalf("got %d anomalies, want 2: %+v", len(got), got)
	}
	// Both have magnitude 8000, so the tie breaks by category name → Dining first.
	if got[0].Category != "Dining" || got[0].Direction != Down {
		t.Errorf("first = %+v, want Dining/Down", got[0])
	}
	if got[1].Category != "Travel" || got[1].Direction != Up {
		t.Errorf("second = %+v, want Travel/Up", got[1])
	}
	if got[0].Delta != -8000 || got[0].PctChange != -80 {
		t.Errorf("Dining delta/pct = %d/%d, want -8000/-80", got[0].Delta, got[0].PctChange)
	}
}
