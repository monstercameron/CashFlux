// SPDX-License-Identifier: MIT

package reports

import (
	"reflect"
	"testing"
)

func TestBestSavingsMonthRequiresPositiveNetAndKeepsFirstTie(t *testing.T) {
	for _, tc := range []struct {
		name  string
		flows []PeriodFlow
		want  int
	}{
		{"empty", nil, -1},
		{"deficit and breakeven", []PeriodFlow{{Expense: 10}, {Income: 10, Expense: 10}}, -1},
		{"net rather than gross income", []PeriodFlow{{Income: 1000, Expense: 990}, {Income: 100, Expense: 20}}, 1},
		{"earliest equal net", []PeriodFlow{{Income: 100}, {Income: 200, Expense: 100}}, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := BestSavingsMonth(tc.flows); got != tc.want {
				t.Fatalf("best = %d, want %d", got, tc.want)
			}
		})
	}
}

func TestTopSpendingCutsUsesComparableMaterialReductionsWithoutMutation(t *testing.T) {
	rows := []CategorySpend{
		{CategoryID: "equal-threshold", Prior: 10000, Amount: 5000, HasDelta: true},
		{CategoryID: "increase", Prior: 10000, Amount: 20000, HasDelta: true},
		{CategoryID: "uncompared", Prior: 100000, Amount: 0},
		{CategoryID: "first-tie", Prior: 20000, Amount: 10000, HasDelta: true},
		{CategoryID: "largest", Prior: 30000, Amount: 5000, HasDelta: true},
		{CategoryID: "second-tie", Prior: 10000, Amount: 0, HasDelta: true},
	}
	before := append([]CategorySpend(nil), rows...)
	got := TopSpendingCuts(rows, 5000, 3)
	if len(got) != 3 || got[0].CategoryID != "largest" || got[1].CategoryID != "first-tie" || got[2].CategoryID != "second-tie" {
		t.Fatalf("cuts = %+v", got)
	}
	if !reflect.DeepEqual(rows, before) {
		t.Fatal("ranking mutated the source rows")
	}
	if got := TopSpendingCuts(rows, 5000, 1); len(got) != 1 || got[0].CategoryID != "largest" {
		t.Fatalf("limited cuts = %+v", got)
	}
	if got := TopSpendingCuts(rows, 5000, 0); got != nil {
		t.Fatalf("zero requested = %+v", got)
	}
}
