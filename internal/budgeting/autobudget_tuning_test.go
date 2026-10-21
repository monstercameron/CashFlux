// SPDX-License-Identifier: MIT

package budgeting

import (
	"errors"
	"math"
	"reflect"
	"testing"

	"github.com/monstercameron/CashFlux/internal/money"
)

func TestScaleAutoBudgetMinorChecksFinalResultAndSupportedRange(t *testing.T) {
	for _, tc := range []struct {
		name    string
		minor   int64
		pct     int
		want    int64
		wantErr bool
	}{
		{"zero percent", 123, 0, 0, false}, {"identity", math.MaxInt64, 100, math.MaxInt64, false},
		{"truncate fractional minor", 123, 5, 6, false}, {"negative truncates toward zero", -123, 5, -6, false},
		{"non-step percentage valid", 123, 17, 20, false}, {"double", 123, 200, 246, false},
		{"product overflow quotient fits", math.MaxInt64 / 2, 200, math.MaxInt64 - 1, false},
		{"quotient overflow", math.MaxInt64, 200, 0, true},
		{"minimum identity", math.MinInt64, 100, math.MinInt64, false},
		{"minimum double overflow", math.MinInt64, 200, 0, true},
		{"negative percent", 100, -1, 0, true}, {"above supported range", 100, 201, 0, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ScaleAutoBudgetMinor(tc.minor, tc.pct)
			if got != tc.want || (err != nil) != tc.wantErr {
				t.Fatalf("got %d, %v; want %d error=%v", got, err, tc.want, tc.wantErr)
			}
			if tc.name == "quotient overflow" && !errors.Is(err, money.ErrOverflow) {
				t.Fatalf("overflow cause lost: %v", err)
			}
		})
	}
}

func TestAutoBudgetTuningSharesRowsTotalsAndPositivePendingLimits(t *testing.T) {
	rows := []BudgetSuggestion{{CategoryID: "one", CategoryName: "One", MonthlyMinor: 123}, {CategoryID: "zero", MonthlyMinor: 50}, {CategoryID: "off", MonthlyMinor: 70}, {CategoryID: "late", MonthlyMinor: 30}}
	percent := map[string]int{"one": 125, "zero": 0, "off": 150}
	selected := map[string]bool{"one": true, "zero": true, "late": true}
	got := PrepareAutoBudgetTuning(rows, percent, selected)
	if got.Err != nil || got.TotalMinor != 183 || got.SelectedCount != 3 || got.TunedCount != 3 || !reflect.DeepEqual(got.PendingLimits, map[string]int64{"one": 153, "late": 30}) {
		t.Fatalf("unexpected prepared selection: %+v", got)
	}
	if got.Rows[0].MonthlyMinor != 153 || got.Rows[1].MonthlyMinor != 0 || got.Rows[2].MonthlyMinor != 105 || got.Rows[3].Percentage != 100 {
		t.Fatalf("row amounts differ from pending selection: %+v", got.Rows)
	}
	// Later user edits must neither alter prepared facts nor be overwritten by preparation.
	if _, exists := percent["late"]; exists {
		t.Fatal("default seeding mutated input percentages")
	}
	rows[0].CategoryName, percent["one"], selected["one"] = "Changed", 200, false
	if got.Rows[0].Suggestion.CategoryName != "One" || got.Rows[0].Percentage != 125 || !got.Rows[0].Selected || got.PendingLimits["one"] != 153 {
		t.Fatal("prepared result retained mutable input state", got)
	}
}

func TestAutoBudgetTuningErrorsDoNotBecomeHealthyZeroOrPartialWrites(t *testing.T) {
	rows := []BudgetSuggestion{{CategoryID: "large", MonthlyMinor: math.MaxInt64}, {CategoryID: "small", MonthlyMinor: 1}}
	for _, tc := range []struct {
		name      string
		pct       map[string]int
		picked    map[string]bool
		wantErr   bool
		wantTotal int64
	}{
		{"selected scale failure", map[string]int{"large": 200}, map[string]bool{"large": true, "small": true}, true, 0},
		{"selected total overflow", nil, map[string]bool{"large": true, "small": true}, true, 0},
		{"unselected bad row leaves healthy selected amount", map[string]int{"large": 200}, map[string]bool{"small": true}, false, 1},
		{"selected invalid percentage", map[string]int{"small": -1}, map[string]bool{"small": true}, true, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := PrepareAutoBudgetTuning(rows, tc.pct, tc.picked)
			if (got.Err != nil) != tc.wantErr || got.TotalMinor != tc.wantTotal {
				t.Fatalf("got %+v", got)
			}
			if tc.wantErr && got.PendingLimits != nil {
				t.Fatal("unavailable selection exposed partial pending writes", got.PendingLimits)
			}
			if tc.name == "unselected bad row leaves healthy selected amount" && (got.Rows[0].Err == nil || len(got.PendingLimits) != 1 || got.PendingLimits["small"] != 1) {
				t.Fatal("row error or independent selected fact lost", got)
			}
		})
	}
	if got := PrepareAutoBudgetTuning(nil, nil, nil); got.Err != nil || got.TotalMinor != 0 || got.SelectedCount != 0 || len(got.Rows) != 0 {
		t.Fatal(got)
	}
}

func TestAutoBudgetTuningKeepsEditsAcrossNewSuggestionsAndMethodChanges(t *testing.T) {
	current := []BudgetSuggestion{{CategoryID: "food", MonthlyMinor: 100}}
	pct := map[string]int{"food": 150}
	picked := map[string]bool{"food": true}
	next := []BudgetSuggestion{{CategoryID: "food", MonthlyMinor: 200}, {CategoryID: "new", MonthlyMinor: 50}}
	first := PrepareAutoBudgetTuning(current, pct, picked)
	second := PrepareAutoBudgetTuning(next, FillAutoBudgetPercentages(pct, next), FillAutoBudgetSelection(picked, next, nil))
	if first.TotalMinor != 150 || second.TotalMinor != 350 || second.Rows[0].Percentage != 150 || !second.Rows[0].Selected || second.Rows[1].Percentage != 100 || !second.Rows[1].Selected {
		t.Fatalf("user tuning or late selection lost: first=%+v next=%+v", first, second)
	}
}
