package reports

import "testing"

func TestTopMovers(t *testing.T) {
	rows := []CategorySpend{
		{CategoryID: "rent", Amount: 90000, Prior: 90000, DeltaPct: 0, HasDelta: true},  // unchanged → excluded
		{CategoryID: "food", Amount: 15000, Prior: 10000, DeltaPct: 50, HasDelta: true}, // +5000
		{CategoryID: "fun", Amount: 0, Prior: 20000, DeltaPct: -100, HasDelta: true},    // -20000 (biggest)
		{CategoryID: "new", Amount: 8000, Prior: 0, HasDelta: false},                    // no delta → excluded
		{CategoryID: "gas", Amount: 12000, Prior: 7000, DeltaPct: 71, HasDelta: true},   // +5000 (tie with food)
	}

	all := TopMovers(rows, 0)
	if len(all) != 3 {
		t.Fatalf("got %d movers, want 3 (food, fun, gas): %+v", len(all), all)
	}
	// fun is the biggest absolute change.
	if all[0].CategoryID != "fun" {
		t.Errorf("top mover = %s, want fun", all[0].CategoryID)
	}
	// food and gas both moved 5000; tie broken by id (food < gas).
	if all[1].CategoryID != "food" || all[2].CategoryID != "gas" {
		t.Errorf("tie order = %s,%s, want food,gas", all[1].CategoryID, all[2].CategoryID)
	}

	top1 := TopMovers(rows, 1)
	if len(top1) != 1 || top1[0].CategoryID != "fun" {
		t.Errorf("TopMovers(n=1) = %+v, want [fun]", top1)
	}
}

func TestTopMoversEmpty(t *testing.T) {
	rows := []CategorySpend{{CategoryID: "x", Amount: 100, Prior: 100, HasDelta: true}}
	if got := TopMovers(rows, 0); len(got) != 0 {
		t.Errorf("got %d, want 0 (nothing changed)", len(got))
	}
}
