// SPDX-License-Identifier: MIT

package txnview

import (
	"math"
	"testing"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func TestSummarizeCountsExcludedRowsOnlyAsVisibleActivity(t *testing.T) {
	rows := []domain.Transaction{
		{ID: "spent", Amount: money.New(-10_000, "USD"), CategoryID: "rent"},
		{ID: "rebate", Amount: money.New(-3_000, "USD"), CategoryID: "rent", ExcludeFromReports: true},
		{ID: "income", Amount: money.New(4_000, "USD")},
	}
	view, err := Summarize(rows, currency.Rates{Base: "USD"}, "USD")
	if err != nil {
		t.Fatal(err)
	}
	if view.Count != 3 || view.Excluded != 1 || view.Spent != 10_000 || view.Income != 4_000 || view.Net != -6_000 {
		t.Fatalf("summary = %+v", view)
	}
	if len(view.Categories) != 1 || view.Categories[0] != (CategoryTotal{ID: "rent", Minor: 10_000}) {
		t.Fatalf("categories = %+v", view.Categories)
	}
}

func TestSummarizeWithCategoriesKeepsFilteredCashflowAndBreakdownScopesDistinct(t *testing.T) {
	filtered := []domain.Transaction{{ID: "groceries", Amount: money.New(-20_000, "USD"), CategoryID: "food"}}
	categoryRows := []domain.Transaction{
		{ID: "groceries", Amount: money.New(-20_000, "USD"), CategoryID: "food"},
		{ID: "rent", Amount: money.New(-150_000, "USD"), CategoryID: "home"},
		{ID: "reimbursement", Amount: money.New(-30_000, "USD"), CategoryID: "home", ExcludeFromReports: true},
	}
	view, err := SummarizeWithCategories(filtered, categoryRows, currency.Rates{Base: "USD"}, "USD")
	if err != nil {
		t.Fatal(err)
	}
	if view.Spent != 20_000 || len(view.Categories) != 2 || view.Categories[0] != (CategoryTotal{ID: "home", Minor: 150_000}) {
		t.Fatalf("view = %+v; filtered Spent should stay $200 while breakdown is $1,700", view)
	}
}

func TestSummarizeTransferPairAndUnpairedFactsOnce(t *testing.T) {
	rows := []domain.Transaction{
		{ID: "out", AccountID: "checking", TransferAccountID: "savings", TransferGroupID: "move", Amount: money.New(-5_000, "USD")},
		{ID: "in", AccountID: "savings", TransferAccountID: "checking", TransferGroupID: "move", Amount: money.New(5_000, "USD")},
		{ID: "orphan", AccountID: "checking", TransferAccountID: "savings", Amount: money.New(-700, "USD")},
	}
	view, err := Summarize(rows, currency.Rates{Base: "USD"}, "USD")
	if err != nil {
		t.Fatal(err)
	}
	if view.Transferred != 5_700 || view.MovedRows != 2 || view.UnpairedRows != 1 {
		t.Fatalf("transfer summary = %+v", view)
	}
	if len(view.CountedTransfers) != len(rows) || !view.CountedTransfers[0] || view.CountedTransfers[1] || !view.CountedTransfers[2] {
		t.Fatalf("counted rows = %v", view.CountedTransfers)
	}
}

func TestSummarizeReportsMissingFXWithoutAddingUnconvertedRows(t *testing.T) {
	rows := []domain.Transaction{{ID: "eur", Amount: money.New(-1_200, "EUR")}, {ID: "usd", Amount: money.New(-500, "USD")}}
	view, err := Summarize(rows, currency.Rates{Base: "USD"}, "USD")
	if err != nil {
		t.Fatal(err)
	}
	if view.Spent != 500 || view.Unconverted != 1 || view.UnconvertedByCurrency["EUR"] != 1 {
		t.Fatalf("summary = %+v", view)
	}
}

func TestSummarizeUsesSplitCategoriesAndChecksOverflow(t *testing.T) {
	rows := []domain.Transaction{{ID: "receipt", Amount: money.New(-5_000, "USD"), Splits: []domain.CategorySplit{
		{CategoryID: "food", Amount: money.New(-3_000, "USD")},
		{CategoryID: "home", Amount: money.New(-2_000, "USD")},
	}}}
	view, err := Summarize(rows, currency.Rates{Base: "USD"}, "USD")
	if err != nil {
		t.Fatal(err)
	}
	if len(view.Categories) != 2 || view.Categories[0] != (CategoryTotal{ID: "food", Minor: 3_000}) {
		t.Fatalf("split categories = %+v", view.Categories)
	}
	_, err = Summarize([]domain.Transaction{
		{ID: "a", Amount: money.New(math.MinInt64, "USD")},
		{ID: "b", Amount: money.New(-1, "USD")},
	}, currency.Rates{Base: "USD"}, "USD")
	if err == nil {
		t.Fatal("overflowing spent sum succeeded")
	}
}

func TestCacheKeysRevisionAndFilterScope(t *testing.T) {
	cache := Cache{}
	rows := []domain.Transaction{{ID: "one", Amount: money.New(-100, "USD")}}
	rates := currency.Rates{Base: "USD"}
	first, err := cache.Get(1, "month", rows, rates, "USD")
	if err != nil {
		t.Fatal(err)
	}
	rows[0].Amount.Amount = -900
	second, err := cache.Get(1, "month", rows, rates, "USD")
	if err != nil {
		t.Fatal(err)
	}
	if second.Spent != first.Spent {
		t.Fatal("same revision/scope did not reuse memoized result")
	}
	third, err := cache.Get(1, "month:category", rows, rates, "USD")
	if err != nil || third.Spent != 900 {
		t.Fatalf("new scope summary = %+v, %v", third, err)
	}
	fourth, err := cache.Get(2, "month:category", rows, rates, "USD")
	if err != nil || fourth.Spent != 900 {
		t.Fatalf("new revision summary = %+v, %v", fourth, err)
	}
	if _, ok := cache.Lookup(2, "month:category"); !ok {
		t.Fatal("cache lookup missed matching revision and scope")
	}
}
