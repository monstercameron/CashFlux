// SPDX-License-Identifier: MIT

package billfit

import (
	"errors"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/bills"
	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func TestPreparedFitIncludesSiblingBillsAndPreservesFirstBudget(t *testing.T) {
	now := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	first := domain.Budget{ID: "first", Name: "First", CategoryID: "cat", Period: domain.PeriodMonthly, Limit: money.New(100, "USD")}
	second := domain.Budget{ID: "second", Name: "Second", CategoryID: "cat", Period: domain.PeriodMonthly, Limit: money.New(1000, "USD")}
	candidate := recurringBill("candidate", "cat", -money.New(60, "USD").Amount, now)
	sibling := recurringBill("sibling", "cat", -30, now)
	p := Prepare(Input{Budgets: []domain.Budget{first, second}, Categories: []domain.Category{{ID: "cat"}},
		Recurring: []domain.Recurring{candidate, sibling}, Rates: currency.Rates{Base: "USD"}, Now: now})
	fact := p.Fit(upcomingRecurring("candidate", 60, now))
	if !fact.HasBudget || !fact.Available || !fact.Fits || fact.BudgetID != "first" || fact.Amount.Amount != 10 {
		t.Fatalf("fit = %+v, want first budget with 10 USD remaining", fact)
	}
}

func TestPreparedFitUsesCheckedSpentAndPendingCurrencyConversion(t *testing.T) {
	now := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	budget := domain.Budget{ID: "b", Name: "Food", CategoryID: "cat", Period: domain.PeriodMonthly, Limit: money.New(100, "USD")}
	candidate := recurringBill("candidate", "cat", -20, now)
	eur := recurringBill("eur", "cat", -5, now)
	eur.Amount.Currency = "EUR"
	p := Prepare(Input{Budgets: []domain.Budget{budget}, Categories: []domain.Category{{ID: "cat"}},
		Recurring: []domain.Recurring{candidate, eur}, Rates: currency.Rates{Base: "USD"}, Now: now})
	fact := p.Fit(upcomingRecurring("candidate", 20, now))
	if !fact.HasBudget || fact.Available || fact.Err == nil || !strings.Contains(fact.Err.Error(), "pending bill") {
		t.Fatalf("missing FX should be unavailable: %+v", fact)
	}

	p = Prepare(Input{Budgets: []domain.Budget{budget}, Categories: []domain.Category{{ID: "cat"}},
		Recurring: []domain.Recurring{candidate}, Rates: currency.Rates{Base: "USD"}, Now: now,
		Ledger: budgeting.NettedLedger{{ID: "spent", Date: now, CategoryID: "cat", Amount: money.New(math.MinInt64, "USD")}}})
	fact = p.Fit(upcomingRecurring("candidate", 20, now))
	if !fact.HasBudget || fact.Available || fact.Err == nil {
		t.Fatalf("overflowing spent should be unavailable: %+v", fact)
	}
}

func TestPreparedFitChecksPendingAdditionAndKeepsPeriodEndExclusive(t *testing.T) {
	now := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	budget := domain.Budget{ID: "b", Name: "Food", CategoryID: "cat", Period: domain.PeriodMonthly, Limit: money.New(100, "USD")}
	candidate := recurringBill("candidate", "cat", -1, now)
	nearMax := recurringBill("max", "cat", -math.MaxInt64, now)
	secondMax := recurringBill("max2", "cat", -math.MaxInt64, now)
	p := Prepare(Input{Budgets: []domain.Budget{budget}, Categories: []domain.Category{{ID: "cat"}},
		Recurring: []domain.Recurring{candidate, nearMax, secondMax}, Rates: currency.Rates{Base: "USD"}, Now: now})
	fact := p.Fit(upcomingRecurring("candidate", 1, now))
	if fact.Available || fact.Err == nil || !strings.Contains(fact.Err.Error(), "sum pending") {
		t.Fatalf("pending overflow should be unavailable: %+v", fact)
	}

	// An occurrence due at the next month boundary must not be reserved in this
	// month's half-open period, matching the existing [start,end) rule.
	endOfMonth := time.Date(2026, 10, 31, 0, 0, 0, 0, time.UTC)
	endCandidate := recurringBill("end", "cat", -20, endOfMonth)
	boundary := recurringBill("boundary", "cat", -50, time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC))
	p = Prepare(Input{Budgets: []domain.Budget{budget}, Categories: []domain.Category{{ID: "cat"}},
		Recurring: []domain.Recurring{endCandidate, boundary}, Rates: currency.Rates{Base: "USD"}, Now: now})
	fact = p.Fit(upcomingRecurring("end", 20, endOfMonth))
	if !fact.Available || !fact.Fits || fact.Amount.Amount != 80 {
		t.Fatalf("period-end bill should ignore next-period pending: %+v", fact)
	}
}

func TestPreparedFitDistinguishesNoChipFromUnavailable(t *testing.T) {
	now := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	b := upcomingRecurring("candidate", 20, now)
	p := Prepare(Input{Now: now})
	if got := p.Fit(b); got.HasBudget || got.Available || got.Err != nil {
		t.Fatalf("no tracked budget should be healthy absence: %+v", got)
	}
	budget := domain.Budget{ID: "b", CategoryID: "cat", Period: domain.PeriodMonthly, Limit: money.New(100, "USD")}
	r := recurringBill("candidate", "cat", -20, now)
	p = Prepare(Input{Budgets: []domain.Budget{budget}, Categories: []domain.Category{{ID: "cat"}},
		Recurring: []domain.Recurring{r}, Rates: currency.Rates{Base: "USD"}, Now: now, SourceError: errors.New("closed store")})
	got := p.Fit(b)
	if !got.HasBudget || got.Available || got.Err == nil {
		t.Fatalf("source error should be unavailable: %+v", got)
	}
	p = Prepare(Input{Now: now, IndexError: errors.New("budget index read failed")})
	got = p.Fit(b)
	if !got.HasBudget || got.Available || got.Err == nil {
		t.Fatalf("index error should not appear as no matching budget: %+v", got)
	}
}

func recurringBill(id, category string, amount int64, due time.Time) domain.Recurring {
	return domain.Recurring{ID: id, Label: id, CategoryID: category, Amount: money.New(amount, "USD"),
		Cadence: domain.CadenceMonthly, NextDue: due}
}

func upcomingRecurring(id string, amount int64, due time.Time) bills.Bill {
	due = time.Date(due.Year(), due.Month(), due.Day(), 0, 0, 0, 0, time.UTC)
	return bills.Bill{AccountID: "recurring:" + id, Amount: money.New(amount, "USD"), DueDate: due}
}
