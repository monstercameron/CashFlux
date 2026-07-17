// SPDX-License-Identifier: MIT

package reports

import (
	"reflect"
	"testing"
)

func TestMonthsNegative(t *testing.T) {
	flows := []PeriodFlow{
		{Income: 1000, Expense: 800}, // +200
		{Income: 500, Expense: 900},  // -400 (red)
		{Income: 0, Expense: 0},      // inactive — skipped
		{Income: 700, Expense: 700},  // 0 (not red)
		{Income: 100, Expense: 300},  // -200 (red)
	}
	if got := MonthsNegative(flows); got != 2 {
		t.Fatalf("MonthsNegative = %d, want 2", got)
	}
	if got := MonthsNegative(nil); got != 0 {
		t.Fatalf("MonthsNegative(nil) = %d, want 0", got)
	}
}

func TestSeasonalExtremes(t *testing.T) {
	flows := []PeriodFlow{
		{Income: 1, Expense: 500},
		{Income: 0, Expense: 0}, // inactive — never an extreme
		{Income: 1, Expense: 900},
		{Income: 1, Expense: 200},
	}
	hi, lo, ok := SeasonalExtremes(flows)
	if !ok || hi != 2 || lo != 3 {
		t.Fatalf("SeasonalExtremes = (%d,%d,%t), want (2,3,true)", hi, lo, ok)
	}
	// One active month → no seasonality.
	if _, _, ok := SeasonalExtremes(flows[:2]); ok {
		t.Fatal("single active month should not report extremes")
	}
}

func TestPer100(t *testing.T) {
	rows := []CategorySpend{
		{CategoryID: "rent", Amount: 30000},  // 30.0 of each 100
		{CategoryID: "food", Amount: -12500}, // 12.5 (negative stored amounts abs'd)
		{CategoryID: "fun", Amount: 5000},    // folded into rest (n=2)
		{CategoryID: "misc", Amount: 2500},   // folded into rest
	}
	got := Per100(rows, 100000, 2)
	want := []Per100Row{
		{CategoryID: "rent", AmountMinor: 30000, Per100: 30, Tenths: 0},
		{CategoryID: "food", AmountMinor: 12500, Per100: 12, Tenths: 5},
		{CategoryID: "", AmountMinor: 7500, Per100: 7, Tenths: 5},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Per100 = %+v, want %+v", got, want)
	}
	if Per100(rows, 0, 2) != nil {
		t.Fatal("zero income must yield nil")
	}
}

func TestTrimTargets(t *testing.T) {
	trends := []CategoryTrend{
		// Creeping: sorted [90 100 100 110 150 150 180] → median 110; recent avg
		// (150+180+150)/3 = 160 → save 50.
		{CategoryID: "dining", Spend: []int64{90, 100, 110, 100, 150, 180, 150}},
		// Flat: no save.
		{CategoryID: "rent", Spend: []int64{500, 500, 500, 500, 500, 500, 500}},
		// Creeping but too small (recent avg < min).
		{CategoryID: "apps", Spend: []int64{1, 1, 1, 1, 5, 5, 5}},
		// Too short a history.
		{CategoryID: "new", Spend: []int64{10, 90, 90}},
	}
	got := TrimTargets(trends, 50, 3)
	if len(got) != 1 || got[0].CategoryID != "dining" {
		t.Fatalf("TrimTargets = %+v, want a single dining target", got)
	}
	if got[0].MedianMinor != 110 || got[0].RecentAvgMinor != 160 || got[0].MonthlySaveMinor != 50 {
		t.Fatalf("dining target = %+v, want median 110 / recent 160 / save 50", got[0])
	}
}

func TestMedianInt64(t *testing.T) {
	if m := medianInt64([]int64{5, 1, 3}); m != 3 {
		t.Fatalf("odd median = %d, want 3", m)
	}
	if m := medianInt64([]int64{4, 1, 3, 2}); m != 2 {
		t.Fatalf("even median = %d, want 2 ((2+3)/2)", m)
	}
}

// TestSeasonalExtremesSkipping locks the QA CF-23 fix: the in-progress month
// (a tiny partial expense) is excluded from the lightest/heaviest ranking.
func TestSeasonalExtremesSkipping(t *testing.T) {
	flows := []PeriodFlow{
		{Income: 100, Expense: 500}, // complete
		{Income: 100, Expense: 900}, // complete, heaviest
		{Income: 100, Expense: 600}, // complete, lightest among complete? no: idx0 is 500
		{Income: 50, Expense: 120},  // partial current month — would win "lightest"
	}
	hi, lo, ok := SeasonalExtremesSkipping(flows, 3)
	if !ok || hi != 1 || lo != 0 {
		t.Errorf("SeasonalExtremesSkipping = hi %d lo %d ok %v, want hi 1 lo 0 ok true", hi, lo, ok)
	}
	// Without a skip the partial month wrongly ranks lightest.
	_, lo2, _ := SeasonalExtremesSkipping(flows, -1)
	if lo2 != 3 {
		t.Errorf("baseline lo = %d, want 3 (documents the pre-fix behavior)", lo2)
	}
}
