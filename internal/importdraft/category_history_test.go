// SPDX-License-Identifier: MIT

package importdraft

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/extract"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/rules"
	"github.com/monstercameron/CashFlux/internal/store"
)

func TestCategoryTextSimilarityUsesNinetyPercentAfterCleaning(t *testing.T) {
	for _, test := range []struct {
		left, right string
		want        int
	}{
		{"merchant01", "merchant02", 90},
		{"merchant01", "merchant99", 0},
		{"AMZN MKTP US*2H4RT9", "AMZN MKTP US*8K1QP2", 100},
		{"APLPAY GREENFIELD MKT #204", "APLPAY GREENFIELD MKT #205", 100},
		{"Coffee", "Coffee", 100},
	} {
		got := categoryTextSimilarity(categoryMatchText(test.left), categoryMatchText(test.right))
		if got != test.want {
			t.Errorf("similarity(%q, %q) = %d, want %d", test.left, test.right, got, test.want)
		}
	}
}

func TestSmartMatchesSampleDataMerchantReference(t *testing.T) {
	sample := store.SampleDatasetAt(time.Date(2026, 10, 19, 0, 0, 0, 0, time.UTC))
	rows := []extract.Row{{Description: "APLPAY GREENFIELD MKT #205", Amount: "-28.50"}}
	got := SuggestLocal(rows, nil, sample.Categories, sample.Transactions, 2)
	if got[0].Category != "Groceries" {
		t.Fatalf("sample merchant category = %q, want Groceries", got[0].Category)
	}
}

func TestSmartHistoryNeedsCloseUnambiguousMatchingDescription(t *testing.T) {
	categories := []domain.Category{
		{ID: "food", Name: "Groceries", Kind: domain.KindExpense},
		{ID: "dining", Name: "Dining", Kind: domain.KindExpense},
		{ID: "pay", Name: "Salary", Kind: domain.KindIncome},
	}
	history := []domain.Transaction{
		{Desc: "merchant01", CategoryID: "food", Amount: money.New(-100, "USD")},
		{Desc: "splitstore", CategoryID: "food", Amount: money.New(-100, "USD")},
		{Desc: "splitstore", CategoryID: "dining", Amount: money.New(-100, "USD")},
		{Desc: "paycompany", CategoryID: "pay", Amount: money.New(100, "USD")},
		{Desc: "excludedxx", CategoryID: "food", Amount: money.New(-100, "USD"), TransferAccountID: "other"},
	}
	rows := []extract.Row{
		{Description: "merchant02", Amount: "-1.00"},                     // 90% similar
		{Description: "merchant99", Amount: "-1.00"},                     // below 90%
		{Description: "splitstore", Amount: "-1.00"},                     // conflicting history
		{Description: "paycompany", Amount: "-1.00"},                     // wrong direction
		{Description: "excludedxx", Amount: "-1.00"},                     // transfer history
		{Description: "merchant02", Amount: "-1.00", Category: "Dining"}, // manual choice
	}
	got := SuggestLocal(rows, nil, categories, history, 2)
	for i, want := range []string{"Groceries", "", "", "", "", "Dining"} {
		if got[i].Category != want {
			t.Errorf("row %d category = %q, want %q", i, got[i].Category, want)
		}
	}
	if rows[0].Category != "" {
		t.Fatal("suggestions mutated the source rows")
	}
	withRule := SuggestLocal(rows[:1], []rules.Rule{{Match: "merchant", SetCategoryID: "dining"}}, categories, history, 2)
	if withRule[0].Category != "Dining" {
		t.Fatalf("user rule did not outrank history: %+v", withRule[0])
	}
}

func TestSmartPlusHistoryHintsAreAggregatedAndIndexed(t *testing.T) {
	categories := []domain.Category{{ID: "food", Name: "Groceries", Kind: domain.KindExpense}, {ID: "dining", Name: "Dining", Kind: domain.KindExpense}}
	history := []domain.Transaction{
		{Desc: "cornerstore", CategoryID: "food", Amount: money.New(-100, "USD")},
		{Desc: "cornerstore", CategoryID: "food", Amount: money.New(-200, "USD")},
		{Desc: "cornerstore", CategoryID: "dining", Amount: money.New(-300, "USD")},
	}
	rows := []extract.Row{{Description: "Already chosen", Category: "Dining", Amount: "-1.00"}, {Description: "cornerstore", Amount: "-2.00"}}
	hints := CategoryHistoryHints(rows, history, categories, 2)
	if len(hints) != 2 || hints[0].Index != 0 || hints[0].Category != "Groceries" || hints[0].Count != 2 || hints[0].Total != 3 || hints[1].Category != "Dining" || hints[1].Count != 1 {
		t.Fatalf("unexpected history hints: %+v", hints)
	}
}
