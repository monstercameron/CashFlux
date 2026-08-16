// SPDX-License-Identifier: MIT

package txnfilter

import (
	"testing"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

// splitSample is a grocery receipt split across two categories plus two plain
// charges, one of them tagged.
func splitSample() []domain.Transaction {
	return []domain.Transaction{
		{
			ID: "receipt", AccountID: "acc1", CategoryID: "shopping", Desc: "Costco",
			Amount: money.New(-12000, "USD"), Date: d("2026-06-05"),
			Splits: []domain.CategorySplit{
				{CategoryID: "groceries", Amount: money.New(-8000, "USD")},
				{CategoryID: "household", Amount: money.New(-4000, "USD")},
			},
		},
		{ID: "gas", AccountID: "acc1", CategoryID: "gas", Desc: "Shell", Amount: money.New(-6000, "USD"), Date: d("2026-06-10")},
		{ID: "flight", AccountID: "acc1", CategoryID: "airfare", Desc: "Airline", Amount: money.New(-30000, "USD"), Date: d("2026-06-12"), Tags: []string{"vacation"}},
	}
}

// A split line's category surfaces its transaction: the budget bar counts that
// line, so the ledger the bar links to must list the receipt (C585).
func TestCategoryFilterMatchesSplitLines(t *testing.T) {
	got := ids(Apply(splitSample(), Criteria{Category: "groceries"}))
	if got != "receipt" {
		t.Errorf("Category=groceries → %q, want \"receipt\" (a split line is a category match)", got)
	}
	// Default order is newest first: the gas charge (06-10), then the receipt (06-05).
	got = ids(Apply(splitSample(), Criteria{Categories: "household,gas"}))
	if got != "gasreceipt" {
		t.Errorf("Categories=household,gas → %q, want the gas charge and the receipt", got)
	}
}

// Splitting a transaction must never make it LESS findable: its own category
// still matches.
func TestCategoryFilterStillMatchesTheParentCategory(t *testing.T) {
	if got := ids(Apply(splitSample(), Criteria{Category: "shopping"})); got != "receipt" {
		t.Errorf("Category=shopping → %q, want \"receipt\"", got)
	}
}

// The budget drill-through scope: in a tracked category OR carrying a tracked
// tag. AND-ing them (the default) would return nothing here, which is exactly
// the empty ledger under a non-zero total that C585 describes.
func TestScopeAnyMatchesCategoryOrTag(t *testing.T) {
	c := Criteria{Categories: "gas", Tags: "vacation", ScopeAny: true}
	got := ids(Apply(splitSample(), c))
	if got != "flightgas" { // newest first: the flight (06-12), then the gas charge (06-10)
		t.Errorf("ScopeAny → %q, want the tagged flight and the gas charge", got)
	}

	andWise := ids(Apply(splitSample(), Criteria{Categories: "gas", Tags: "vacation"}))
	if andWise != "" {
		t.Errorf("without ScopeAny the same selection is an AND and must return nothing; got %q", andWise)
	}
}

func TestScopeAnyWithOnlyCategoriesBehavesLikeACategoryFilter(t *testing.T) {
	plain := ids(Apply(splitSample(), Criteria{Categories: "groceries"}))
	scoped := ids(Apply(splitSample(), Criteria{Categories: "groceries", ScopeAny: true}))
	if plain != scoped {
		t.Errorf("ScopeAny changed a categories-only filter: %q vs %q", plain, scoped)
	}
}

func TestScopeAnyWithNothingSelectedFiltersNothing(t *testing.T) {
	all := ids(Apply(splitSample(), Criteria{}))
	scoped := ids(Apply(splitSample(), Criteria{ScopeAny: true}))
	if all != scoped {
		t.Errorf("ScopeAny with no category or tag narrowed the set: %q vs %q", all, scoped)
	}
}

// ScopeAny only reshapes the category/tag dimensions; every other filter still
// ANDs, so a date window still applies.
func TestScopeAnyStillAndsOtherDimensions(t *testing.T) {
	c := Criteria{Categories: "gas", Tags: "vacation", ScopeAny: true, From: "2026-06-11", To: "2026-06-30"}
	if got := ids(Apply(splitSample(), c)); got != "flight" {
		t.Errorf("ScopeAny with a date window → %q, want \"flight\"", got)
	}
}
