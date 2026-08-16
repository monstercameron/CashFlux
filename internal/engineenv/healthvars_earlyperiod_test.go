// SPDX-License-Identifier: MIT

package engineenv

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/healthscore"
	"github.com/monstercameron/CashFlux/internal/money"
)

// earlyPeriodData builds a household with ONE budget that was blown last month
// and has had nothing spent against it yet this month.
//
// Read on the current period, adherence is a perfect 100%: no budget has been
// broken, because the month has barely begun. That was the C344 reading — a
// confident "excellent" that is a statement about the calendar. Read on the last
// COMPLETED period, it is 0%, which is the true state of this household.
func earlyPeriodData(now time.Time) Data {
	usd := func(minor int64) money.Money { return money.New(minor, "USD") }
	lastMonth := now.AddDate(0, -1, 0)
	return Data{
		Accounts: []domain.Account{{
			ID: "chk", Class: domain.ClassAsset, Type: domain.TypeChecking, Currency: "USD",
			OpeningBalance: usd(500_000), BalanceAsOf: now.AddDate(0, -6, 0),
		}},
		Categories: []domain.Category{{ID: "cat-food", Name: "Groceries", Kind: domain.KindExpense}},
		Budgets: []domain.Budget{{
			ID: "bud-food", CategoryID: "cat-food", Limit: usd(20_000), Period: domain.PeriodMonthly,
		}},
		Transactions: []domain.Transaction{
			// Last month: $400 against a $200 limit — comfortably over.
			{ID: "t-over", AccountID: "chk", CategoryID: "cat-food",
				Date:   time.Date(lastMonth.Year(), lastMonth.Month(), 12, 0, 0, 0, 0, time.UTC),
				Amount: usd(-40_000)},
			// Income so the model has enough applicable factors to be worth reading.
			{ID: "i1", AccountID: "chk", Date: now.AddDate(0, -1, 0), Amount: usd(300_000)},
			{ID: "i2", AccountID: "chk", Date: now.AddDate(0, -2, 0), Amount: usd(300_000)},
			{ID: "i3", AccountID: "chk", Date: now.AddDate(0, -3, 0), Amount: usd(300_000)},
		},
		Rates: currency.Rates{Base: "USD"},
		Now:   now,
	}
}

// TestBudgetAdherenceFallsBackWhileThePeriodIsYoung is C344 on /health.
func TestBudgetAdherenceFallsBackWhileThePeriodIsYoung(t *testing.T) {
	// Day 2 of a 31-day month: ~3% elapsed, well inside "too young to read".
	early := HealthInputs(earlyPeriodData(time.Date(2026, time.July, 2, 9, 0, 0, 0, time.UTC)))
	if !early.HasBudgets {
		t.Fatal("fixture should have a budget")
	}
	if early.BudgetAdherencePct != 0 {
		t.Errorf("adherence on day 2 = %d%%, want 0%% — scoring the current period there "+
			"reports a perfect month because nothing has happened in it yet (C344)",
			early.BudgetAdherencePct)
	}
	if early.BudgetAdherenceWindow != healthscore.WindowPriorPeriod {
		t.Errorf("window = %q, want %q — a fallback the UI cannot see is a different lie",
			early.BudgetAdherenceWindow, healthscore.WindowPriorPeriod)
	}

	// The factor the model produces must carry the same window through.
	r := healthscore.Evaluate(early)
	for _, f := range r.Factors {
		if f.Key != "budget" {
			continue
		}
		if f.Window != healthscore.WindowPriorPeriod {
			t.Errorf("budget factor window = %q, want %q", f.Window, healthscore.WindowPriorPeriod)
		}
	}
}

// TestBudgetAdherenceReadsTheCurrentPeriodOnceItHasRun is the other half: the
// fallback must step aside, or /health would permanently report stale figures.
func TestBudgetAdherenceReadsTheCurrentPeriodOnceItHasRun(t *testing.T) {
	// Day 20: the month is real now, and nothing has been spent against the
	// budget this month, so adherence is genuinely 100%.
	late := HealthInputs(earlyPeriodData(time.Date(2026, time.July, 20, 9, 0, 0, 0, time.UTC)))
	if late.BudgetAdherencePct != 100 {
		t.Errorf("adherence on day 20 = %d%%, want 100%% — the current period is the "+
			"right window once there is enough of it to read", late.BudgetAdherencePct)
	}
	if late.BudgetAdherenceWindow != healthscore.WindowCurrentPeriod {
		t.Errorf("window = %q, want %q", late.BudgetAdherenceWindow, healthscore.WindowCurrentPeriod)
	}
}
