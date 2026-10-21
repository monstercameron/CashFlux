// SPDX-License-Identifier: MIT

package categoryview

import (
	"errors"
	"math"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func TestPreparePeriodUsesCanonicalRefundSplitAndIncomePolicies(t *testing.T) {
	start := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	end := start.AddDate(0, 1, 0)
	transactions := []domain.Transaction{
		{ID: "buy", Date: start, CategoryID: "food", Amount: money.New(-10000, "USD")},
		{ID: "refund", Date: end, CategoryID: "food", Amount: money.New(4000, "USD")},
		{ID: "split-spend", Date: start, CategoryID: "parent", Amount: money.New(-9000, "USD"), Splits: []domain.CategorySplit{
			{CategoryID: "food", Amount: money.New(-3000, "USD")},
			{CategoryID: "", Amount: money.New(-1000, "USD")},
			{CategoryID: "credit-line", Amount: money.New(1000, "USD")},
		}},
		{ID: "pay", Date: start, CategoryID: "salary", Amount: money.New(10000, "USD"), Splits: []domain.CategorySplit{
			{CategoryID: "salary", Amount: money.New(4000, "USD")},
			{CategoryID: "bonus", Amount: money.New(3000, "USD")},
		}},
		{ID: "foreign", Date: start, CategoryID: "travel", Amount: money.New(-500, "EUR")},
	}
	links := []domain.TxnLink{{Kind: domain.TxnLinkRefundPair, TxnIDs: []string{"buy", "refund"}, Amount: money.New(4000, "USD")}}
	facts := PreparePeriod(PeriodInput{Transactions: transactions, TxnLinks: links}, start, end, currency.Rates{Base: "USD", Rates: map[string]float64{"EUR": 2}})
	if facts.TxnCountByCategory["food"] != 2 || facts.TxnCountByCategory["salary"] != 1 {
		t.Fatalf("whole-transaction usage counts = %#v", facts.TxnCountByCategory)
	}
	if !facts.SpendingAvailable || !facts.TotalSpendAvailable || facts.TotalSpend != 11000 || facts.UnfiledSpend != 1000 || facts.MaxSpend != 9000 || facts.TopSpendCategoryID != "food" {
		t.Fatalf("spending facts = %+v", facts)
	}
	if facts.SpendByCategory["food"] != 9000 || facts.SpendByCategory[""] != 1000 || facts.SpendByCategory["travel"] != 1000 || facts.SpendByCategory["credit-line"] != 0 {
		t.Fatalf("spend by category = %#v", facts.SpendByCategory)
	}
	if !facts.IncomeAvailable || facts.IncomeByCategory["salary"] != 7000 || facts.IncomeByCategory["bonus"] != 3000 || facts.MaxIncome != 7000 {
		t.Fatalf("income facts = %+v", facts)
	}
	if facts.SpendSharePctByCategory["food"] != 100 || facts.SpendSharePctByCategory[""] != 0 || facts.IncomeSharePctByCategory["salary"] != 100 || facts.IncomeSharePctByCategory["bonus"] != 42 {
		t.Fatalf("share facts spend=%#v income=%#v", facts.SpendSharePctByCategory, facts.IncomeSharePctByCategory)
	}
	if transactions[0].Amount.Amount != -10000 {
		t.Fatal("refund preparation mutated the source transactions")
	}
}

func TestPreparePeriodKeepsIndependentFamiliesWhenFXIsUnavailable(t *testing.T) {
	start := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	end := start.AddDate(0, 1, 0)
	transactions := []domain.Transaction{
		{Date: start, CategoryID: "foreign-expense", Amount: money.New(-100, "EUR")},
		{Date: start, CategoryID: "salary", Amount: money.New(500, "USD")},
	}
	facts := PreparePeriod(PeriodInput{Transactions: transactions}, start, end, currency.Rates{Base: "USD"})
	if facts.TxnCountByCategory["foreign-expense"] != 1 || !facts.IncomeAvailable || facts.IncomeByCategory["salary"] != 500 {
		t.Fatalf("independent count/income facts were lost: %+v", facts)
	}
	if facts.SpendingAvailable || facts.TotalSpendAvailable || facts.SpendingErr == nil || facts.SpendByCategory != nil {
		t.Fatalf("missing expense FX became a plausible amount: %+v", facts)
	}
}

func TestPreparePeriodKeepsIncomeAndCountsWhenRefundLinksCannotBeRead(t *testing.T) {
	start := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	end := start.AddDate(0, 1, 0)
	transactions := []domain.Transaction{
		{Date: start, CategoryID: "expense", Amount: money.New(-250, "USD")},
		{Date: start, CategoryID: "income", Amount: money.New(500, "USD")},
	}
	facts := PreparePeriod(PeriodInput{Transactions: transactions, TxnLinksErr: errors.New("links unavailable")}, start, end, currency.Rates{Base: "USD"})
	if facts.TxnCountByCategory["expense"] != 1 || !facts.IncomeAvailable || facts.IncomeByCategory["income"] != 500 {
		t.Fatalf("independent count/income facts were lost: %+v", facts)
	}
	if facts.SpendingAvailable || facts.SpendingErr == nil || facts.SpendByCategory != nil {
		t.Fatalf("unknown refund links became a trusted spend result: %+v", facts)
	}
}

func TestPreparePeriodSeparatesRowFactsFromTotalOverflow(t *testing.T) {
	start := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	end := start.AddDate(0, 1, 0)
	transactions := []domain.Transaction{
		{Date: start, CategoryID: "a", Amount: money.New(-math.MaxInt64, "USD")},
		{Date: start, CategoryID: "b", Amount: money.New(-1, "USD")},
		{Date: start, CategoryID: "income", Amount: money.New(100, "USD")},
	}
	facts := PreparePeriod(PeriodInput{Transactions: transactions}, start, end, currency.Rates{Base: "USD"})
	if !facts.SpendingAvailable || facts.SpendByCategory["a"] != math.MaxInt64 || facts.SpendByCategory["b"] != 1 {
		t.Fatalf("valid category rows were discarded: %+v", facts)
	}
	if facts.TotalSpendAvailable || facts.TotalSpendErr == nil {
		t.Fatalf("overflowing headline total became available: %+v", facts)
	}
	if !facts.IncomeAvailable || facts.IncomeByCategory["income"] != 100 {
		t.Fatalf("independent income was discarded: %+v", facts)
	}
}

func TestPreparePeriodKeepsUnfiledSpendOutOfCategoryShareScale(t *testing.T) {
	start := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	end := start.AddDate(0, 1, 0)
	transactions := []domain.Transaction{
		{Date: start, CategoryID: "small", Amount: money.New(-100, "USD")},
		{Date: start, CategoryID: "", Amount: money.New(-500, "USD")},
	}
	facts := PreparePeriod(PeriodInput{Transactions: transactions}, start, end, currency.Rates{Base: "USD"})
	if facts.TotalSpend != 600 || facts.UnfiledSpend != 500 || facts.MaxSpend != 100 || facts.TopSpendCategoryID != "small" || facts.SpendSharePctByCategory["small"] != 100 {
		t.Fatalf("unfiled spend changed the category scale: %+v", facts)
	}
}

func TestPreparePeriodPreparesLifetimeUsageOnceWithLegacyBudgetSemantics(t *testing.T) {
	start := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	input := PeriodInput{
		Transactions: []domain.Transaction{
			{ID: "past", CategoryID: "food", Date: start.AddDate(-4, 0, 0)},
			{ID: "current", CategoryID: "food", Date: start},
		},
		Budgets: []domain.Budget{{ID: "multi", CategoryID: "food", CategoryIDs: []string{"food", "travel"}}},
	}
	facts := PreparePeriod(input, start, start.AddDate(0, 1, 0), currency.Rates{Base: "USD"})
	if got := facts.LifetimeUsageByCategory["food"]; got != 3 {
		t.Fatalf("lifetime usage = %d, want two transaction refs and one primary budget ref", got)
	}
	if got := facts.LifetimeUsageByCategory["travel"]; got != 0 {
		t.Fatalf("multi-category secondary ref changed legacy lifetime usage: %d", got)
	}
}
