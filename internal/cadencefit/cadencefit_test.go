// SPDX-License-Identifier: MIT

package cadencefit

import "testing"

// series builds 12 periods with the given amount in the listed indexes.
func series(amount int64, at ...int) []int64 {
	out := make([]int64, 12)
	for _, i := range at {
		out[i] = amount
	}
	return out
}

func TestAnAnnualBillOnAMonthlyBudget(t *testing.T) {
	f := Assess(series(60_000, 3))
	if !f.Known {
		t.Fatal("expected a verdict")
	}
	if f.Shape != ShapeAnnual {
		t.Errorf("shape = %q, want %q", f.Shape, ShapeAnnual)
	}
	if !f.Mismatched() {
		t.Error("a once-a-year bill on a monthly budget was not flagged")
	}
	// The remedy is the sinking fund, so the figure has to be the monthly
	// set-aside rather than the bill.
	if f.SuggestedMonthlyMinor != 5_000 {
		t.Errorf("monthly = %d, want 5000 (a twelfth of the year)", f.SuggestedMonthlyMinor)
	}
	if f.ActivePeriods != 1 || f.Periods != 12 {
		t.Errorf("evidence = %d/%d periods", f.ActivePeriods, f.Periods)
	}
}

func TestQuarterlySpendingIsNamedAsSuch(t *testing.T) {
	f := Assess(series(30_000, 0, 3, 6, 9))
	if f.Shape != ShapeQuarterly {
		t.Errorf("shape = %q, want %q", f.Shape, ShapeQuarterly)
	}
	if !f.Mismatched() {
		t.Error("quarterly spending on a monthly budget was not flagged")
	}
}

// Groceries vary month to month and belong on a monthly budget. The signal is
// concentration, not variability.
func TestVariableButMonthlySpendingIsLeftAlone(t *testing.T) {
	lumpy := []int64{40_000, 12_000, 90_000, 30_000, 5_000, 60_000, 25_000, 80_000, 10_000, 45_000, 33_000, 70_000}
	f := Assess(lumpy)
	if f.Shape != ShapeOrdinary {
		t.Errorf("shape = %q, want %q — wildly varying monthly spend still fits a monthly budget", f.Shape, ShapeOrdinary)
	}
	if f.Mismatched() {
		t.Error("ordinary monthly spending was flagged as a cadence mismatch")
	}
}

// Four payments in a fortnight are one event that happened to span a boundary.
// Calling that quarterly would have somebody set money aside for a repeat that
// is not coming.
func TestABurstIsNotACadence(t *testing.T) {
	f := Assess(series(30_000, 5, 6, 7))
	if f.Shape == ShapeQuarterly {
		t.Errorf("three consecutive months were called quarterly: %+v", f)
	}
	if f.Shape != ShapeIrregular {
		t.Errorf("shape = %q, want %q", f.Shape, ShapeIrregular)
	}
}

// One payment in the six months on record is one payment, not a yearly bill.
func TestTooLittleHistoryIsNotAnAnnualBill(t *testing.T) {
	short := make([]int64, MinPeriods-1)
	short[2] = 60_000
	f := Assess(short)
	if f.Known {
		t.Errorf("a verdict was reached from %d periods: %+v", len(short), f)
	}
	if f.Mismatched() {
		t.Error("an unknown fit was reported as a mismatch")
	}
}

// Nobody should reshape a budget over $40 a year.
func TestSmallMoneyIsNotWorthReshapingABudgetOver(t *testing.T) {
	f := Assess(series(4_000, 3))
	if f.Shape != ShapeAnnual {
		t.Fatalf("shape = %q, want the rhythm still named", f.Shape)
	}
	if f.Mismatched() {
		t.Errorf("a $40/yr category was flagged: total=%d", f.TotalMinor)
	}
}

// A budget with nothing in it is a different problem, and not this one.
func TestAnEmptyBudgetIsNotAMismatch(t *testing.T) {
	f := Assess(make([]int64, 12))
	if !f.Known {
		t.Fatal("expected a verdict")
	}
	if f.Shape != ShapeOrdinary || f.Mismatched() {
		t.Errorf("an unused budget was reported as a cadence problem: %+v", f)
	}
}

// The empty periods ARE the signal — a caller passing only the periods with
// spending would describe every category as annual, so the doc says so and this
// pins the behaviour it depends on.
func TestEmptyPeriodsCount(t *testing.T) {
	dense := Assess([]int64{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1})
	if dense.ActivePeriods != 12 {
		t.Errorf("active = %d, want 12", dense.ActivePeriods)
	}
	sparse := Assess(series(60_000, 6))
	if sparse.ActivePeriods != 1 {
		t.Errorf("active = %d, want 1", sparse.ActivePeriods)
	}
}

func TestABillThatSlippedAMonthIsStillQuarterly(t *testing.T) {
	// Gaps of 3, 4, 3 — a real bill, paid a little late once.
	f := Assess(series(30_000, 0, 3, 7, 10))
	if f.Shape != ShapeQuarterly {
		t.Errorf("shape = %q, want %q — a one-month slip is not irregularity", f.Shape, ShapeQuarterly)
	}
}

func TestTwiceAYearReadsAsAnnual(t *testing.T) {
	f := Assess(series(50_000, 1, 7))
	if f.Shape != ShapeAnnual {
		t.Errorf("shape = %q, want %q", f.Shape, ShapeAnnual)
	}
	if f.SuggestedMonthlyMinor == 0 {
		t.Error("no set-aside figure was offered")
	}
}

func TestAssessIsStable(t *testing.T) {
	in := series(30_000, 0, 3, 6, 9)
	first := Assess(in)
	for i := range 5 {
		if got := Assess(in); got != first {
			t.Fatalf("run %d differed: %+v vs %+v", i, got, first)
		}
	}
}
