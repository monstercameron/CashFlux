// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"testing"

	"github.com/monstercameron/CashFlux/internal/catsuggest"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/reviewqueue"
)

func TestReviewGroupSuggestionRequiresCompatiblePerTransactionResults(t *testing.T) {
	items := []domain.Transaction{{ID: "eur", AccountID: "eur-card"}, {ID: "usd", AccountID: "usd-card"}}
	cases := []struct {
		name     string
		byID     map[string]catsuggest.Suggestion
		want     bool
		conflict bool
	}{
		{"same category", map[string]catsuggest.Suggestion{"eur": {CategoryID: "travel"}, "usd": {CategoryID: "travel"}}, true, false},
		{"account-specific categories", map[string]catsuggest.Suggestion{"eur": {CategoryID: "travel"}, "usd": {CategoryID: "food"}}, false, true},
		{"only one account matches", map[string]catsuggest.Suggestion{"eur": {CategoryID: "travel"}}, false, true},
		{"no account matches", map[string]catsuggest.Suggestion{}, false, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, got, conflict := sharedReviewSuggestion(reviewqueue.Group{Items: items}, tc.byID)
			if got != tc.want {
				t.Fatalf("shared suggestion available = %v, want %v", got, tc.want)
			}
			if conflict != tc.conflict {
				t.Fatalf("suggestion conflict = %v, want %v", conflict, tc.conflict)
			}
		})
	}
}

func TestUnbalancedSplitCannotUseCategoryAsResolution(t *testing.T) {
	unbalanced := domain.Transaction{Amount: money.New(-1000, "USD"), Splits: []domain.CategorySplit{
		{CategoryID: "food", Amount: money.New(-400, "USD")},
	}}
	if reviewqueue.CanAssignCategory(unbalanced) {
		t.Fatal("unbalanced split still offers categorization as a resolution")
	}
	if !reviewqueue.CanAssignCategory(domain.Transaction{Amount: money.New(-1000, "USD")}) {
		t.Fatal("ordinary uncategorized charge should remain categorizable")
	}
}

func TestTxnCatSuggestInputsCacheBuildsOncePerRevision(t *testing.T) {
	var cache txnCatSuggestInputsCache
	builds := 0
	build := func() txnCatSuggestInputs {
		builds++
		return txnCatSuggestInputs{byID: map[string]domain.Transaction{"one": {ID: "one"}}}
	}
	for i := 0; i < 319; i++ {
		cache.get(nil, 7, build)
	}
	if builds != 1 {
		t.Fatalf("shared input builds for 319 rows = %d, want 1", builds)
	}
	cache.get(nil, 8, build)
	if builds != 2 {
		t.Fatalf("shared input builds after revision change = %d, want 2", builds)
	}
}

func TestReviewCurrencyTotalsDoNotCombineDifferentCurrencies(t *testing.T) {
	txns := []domain.Transaction{
		{Amount: money.New(-2000, "USD")},
		{Amount: money.New(-2000, "EUR")},
	}
	if got, want := reviewCurrencyTotals(txns), "−€20.00 EUR · −$20.00 USD"; got != want {
		t.Fatalf("reviewCurrencyTotals() = %q, want %q", got, want)
	}
}
