// SPDX-License-Identifier: MIT

package assistantview

import (
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func questionTxn(id, category, currencyCode string, amount int64, date time.Time) domain.Transaction {
	return domain.Transaction{ID: id, AccountID: "checking", CategoryID: category, Payee: id, Amount: money.New(amount, currencyCode), Date: date}
}

func TestQuestionsRanksConvertibleReportedExpenses(t *testing.T) {
	now := time.Date(2026, 10, 19, 12, 0, 0, 0, time.UTC)
	start, end := dateutil.MonthRange(now)
	txns := []domain.Transaction{
		questionTxn("groceries", "food", "USD", -10000, now),
		questionTxn("euro groceries", "food", "EUR", -10000, now),
		questionTxn("utilities", "bills", "USD", -15000, now),
		questionTxn("missing FX", "", "JPY", -1000000, now),
		questionTxn("past", "bills", "USD", -900000, start.AddDate(0, 0, -1)),
		questionTxn("future", "bills", "USD", -900000, end),
		questionTxn("income", "bills", "USD", 900000, now),
	}
	excluded := questionTxn("excluded", "bills", "USD", -900000, now)
	excluded.ExcludeFromReports = true
	transfer := questionTxn("transfer", "bills", "USD", -900000, now)
	transfer.TransferAccountID = "savings"
	txns = append(txns, excluded, transfer)
	categories := []domain.Category{{ID: "food", Name: "Food"}, {ID: "bills", Name: "Bills"}}
	got := Questions(txns, categories, nil, nil, currency.Rates{Base: "USD", Rates: map[string]float64{"EUR": 2}}, start, end, now)
	if got.TopCategory != "Food" || got.LargestExpense.ID != "euro groceries" || got.LargestExpenseMinor != 20000 || got.UncategorizedCount != 1 {
		t.Fatalf("incorrect facts: %+v", got)
	}
}

func TestQuestionsUsesSplitCategoriesAndViewOrder(t *testing.T) {
	now := time.Date(2026, 10, 19, 12, 0, 0, 0, time.UTC)
	start, end := dateutil.MonthRange(now)
	transaction := questionTxn("split", "", "USD", -10000, now)
	transaction.Splits = []domain.CategorySplit{
		{CategoryID: "food", Amount: money.New(-6000, "USD")},
		{CategoryID: "bills", Amount: money.New(-5000, "USD")},
		{CategoryID: "refund", Amount: money.New(1000, "USD")},
	}
	statuses := []budgeting.Status{
		{Budget: domain.Budget{Name: "First near"}, State: budgeting.StateNear},
		{Budget: domain.Budget{Name: "Second over"}, State: budgeting.StateOver},
		{Budget: domain.Budget{Name: "Third over"}, State: budgeting.StateOver},
	}
	goals := []domain.Goal{
		{Name: "Archived", TargetDate: now.Add(time.Hour), Archived: true},
		{Name: "Past", TargetDate: now.Add(-time.Hour)},
		{Name: "Undated"},
		{Name: "Later", TargetDate: now.AddDate(0, 2, 0)},
		{Name: "Soon", TargetDate: now.AddDate(0, 1, 0)},
	}
	got := Questions([]domain.Transaction{transaction}, []domain.Category{{ID: "food", Name: "Food"}, {ID: "bills", Name: "Bills"}}, statuses, goals, currency.Rates{Base: "USD"}, start, end, now)
	if got.TopCategory != "Food" || got.UncategorizedCount != 0 || got.NearLimitBudget != "First near" || got.OverBudgetCount != 2 || got.UpcomingGoal != "Soon" {
		t.Fatalf("incorrect facts: %+v", got)
	}
	if transaction.Splits[0].Amount.Amount != -6000 {
		t.Fatal("input was mutated")
	}
}

func TestQuestionsCategoryTiesAndInvalidMagnitude(t *testing.T) {
	now := time.Date(2026, 10, 19, 0, 0, 0, 0, time.UTC)
	start, end := dateutil.MonthRange(now)
	txns := []domain.Transaction{questionTxn("a", "a", "USD", -100, now), questionTxn("b", "b", "USD", -100, now), questionTxn("overflow", "a", "USD", math.MinInt64, now)}
	got := Questions(txns, []domain.Category{{ID: "b", Name: "B"}, {ID: "a", Name: "A"}}, nil, nil, currency.Rates{Base: "USD"}, start, end, now)
	if got.TopCategory != "B" || got.LargestExpense.ID != "a" {
		t.Fatalf("ties must retain input order and invalid magnitudes must be skipped: %+v", got)
	}
}

func TestQuestionsCountsPartlyUncategorizedSplitReceiptsOnce(t *testing.T) {
	now := time.Date(2026, 10, 19, 0, 0, 0, 0, time.UTC)
	start, end := dateutil.MonthRange(now)
	partial := questionTxn("partial", "food", "USD", -100, now)
	partial.Splits = []domain.CategorySplit{
		{CategoryID: "food", Amount: money.New(-50, "USD")},
		{Amount: money.New(-25, "USD")},
		{Amount: money.New(-25, "USD")},
	}
	complete := questionTxn("complete", "", "USD", -100, now)
	complete.Splits = []domain.CategorySplit{
		{CategoryID: "food", Amount: money.New(-150, "USD")},
		{Amount: money.New(50, "USD")},
	}
	got := Questions([]domain.Transaction{partial, complete}, nil, nil, nil, currency.Rates{Base: "USD"}, start, end, now)
	if got.UncategorizedCount != 1 {
		t.Fatalf("uncategorized receipt count = %d, want 1", got.UncategorizedCount)
	}
}

func BenchmarkQuestions100K(b *testing.B) {
	now := time.Date(2026, 10, 19, 0, 0, 0, 0, time.UTC)
	start, end := dateutil.MonthRange(now)
	txns := make([]domain.Transaction, 100000)
	categories := make([]domain.Category, 20)
	for i := range categories {
		categories[i] = domain.Category{ID: fmt.Sprint(i), Name: fmt.Sprint(i)}
	}
	for i := range txns {
		txns[i] = questionTxn(fmt.Sprint(i), fmt.Sprint(i%len(categories)), "USD", -int64(i%10000+1), now.AddDate(0, 0, -(i%90)))
	}
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		Questions(txns, categories, nil, nil, currency.Rates{Base: "USD"}, start, end, now)
	}
}
