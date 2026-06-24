// SPDX-License-Identifier: MIT

package reports

import (
	"strconv"
	"strings"
	"testing"
)

// fmtDollars renders minor units as whole dollars for readable test assertions.
func fmtDollars(v int64) string { return "$" + strconv.FormatInt(v/100, 10) }

// names maps a couple of category ids; unknown ids return "".
func names(id string) string {
	return map[string]string{"food": "Food", "rent": "Rent", "fun": "Fun"}[id]
}

func TestSpendingNarrativeEmpty(t *testing.T) {
	if got := SpendingNarrative(nil, false, fmtDollars, names); got != "No spending in this period." {
		t.Errorf("empty narrative = %q", got)
	}
	// Rows that are all zero-amount movers also read as no spending.
	rows := []CategorySpend{{CategoryID: "fun", Amount: 0, Prior: 5000, DeltaPct: -100, HasDelta: true}}
	if got := SpendingNarrative(rows, true, fmtDollars, names); got != "No spending in this period." {
		t.Errorf("all-zero narrative = %q", got)
	}
}

func TestSpendingNarrativeNoComparison(t *testing.T) {
	rows := []CategorySpend{
		{CategoryID: "rent", Amount: 90000},
		{CategoryID: "food", Amount: 15000},
	}
	got := SpendingNarrative(rows, false, fmtDollars, names)
	if !strings.Contains(got, "You spent $1050 across 2 categories.") {
		t.Errorf("missing headline: %q", got)
	}
	if !strings.Contains(got, "Your biggest expense was Rent at $900.") {
		t.Errorf("missing biggest: %q", got)
	}
	if strings.Contains(got, "versus the prior period") {
		t.Errorf("should not mention prior without comparison: %q", got)
	}
}

func TestSpendingNarrativeSingularAndUncategorized(t *testing.T) {
	rows := []CategorySpend{{CategoryID: "", Amount: 5000}}
	got := SpendingNarrative(rows, false, fmtDollars, names)
	if !strings.Contains(got, "across 1 category.") {
		t.Errorf("expected singular category: %q", got)
	}
	if !strings.Contains(got, "was uncategorized at $50.") {
		t.Errorf("expected uncategorized label: %q", got)
	}
}

func TestSpendingNarrativeTopMover(t *testing.T) {
	rows := []CategorySpend{
		{CategoryID: "rent", Amount: 90000, Prior: 90000, DeltaPct: 0, HasDelta: true}, // no change
		{CategoryID: "food", Amount: 15000, Prior: 10000, DeltaPct: 50, HasDelta: true},
		{CategoryID: "fun", Amount: 0, Prior: 20000, DeltaPct: -100, HasDelta: true}, // biggest absolute drop
	}
	got := SpendingNarrative(rows, true, fmtDollars, names)
	// Fun changed by 20000 (largest), down 100%.
	if !strings.Contains(got, "Fun fell 100% to $0 versus the prior period.") {
		t.Errorf("expected Fun as top mover: %q", got)
	}
}
