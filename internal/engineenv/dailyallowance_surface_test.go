// SPDX-License-Identifier: MIT

package engineenv

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

// TestDailyAllowanceVars pins the BG8 atoms + molecule: days_left counts the days
// remaining in the active period, remaining_discretionary sums money left across
// budgets, and daily_allowance divides the two (safediv → 0 once the period ends).
func TestDailyAllowanceVars(t *testing.T) {
	// Active period: June 2026; now is the 21st, so 10 days remain to July 1.
	start := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	now := time.Date(2026, 6, 21, 0, 0, 0, 0, time.UTC)

	budget := domain.Budget{
		ID: "b1", Name: "Dining", CategoryID: "food",
		Period: domain.PeriodMonthly, Limit: money.New(30000, "USD"), // $300
	}
	txns := []domain.Transaction{
		{Amount: money.New(-10000, "USD"), CategoryID: "food", Date: now}, // spent $100 → $200 left
	}

	vars := Vars(Data{
		Budgets: []domain.Budget{budget}, Transactions: txns,
		Rates: currency.Rates{Base: "USD"}, Now: now, PeriodStart: start, PeriodEnd: end,
	})

	if got := vars["days_left"]; got != 10 {
		t.Errorf("days_left = %v, want 10", got)
	}
	if got := vars["remaining_discretionary"]; got != 200 {
		t.Errorf("remaining_discretionary = %v, want 200", got)
	}
	// daily_allowance = 200 / 10 = 20.
	if got := vars["daily_allowance"]; got != 20 {
		t.Errorf("daily_allowance = %v, want 20", got)
	}
}

// TestDailyAllowanceEndedPeriod verifies safediv guards a zero days_left: an ended
// period yields a $0/day allowance rather than a divide-by-zero.
func TestDailyAllowanceEndedPeriod(t *testing.T) {
	start := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	now := time.Date(2026, 7, 5, 0, 0, 0, 0, time.UTC) // period already over

	vars := Vars(Data{
		Rates: currency.Rates{Base: "USD"}, Now: now, PeriodStart: start, PeriodEnd: end,
	})
	if got := vars["days_left"]; got != 0 {
		t.Errorf("days_left = %v, want 0", got)
	}
	if got := vars["daily_allowance"]; got != 0 {
		t.Errorf("daily_allowance = %v, want 0 (safediv fallback)", got)
	}
}
