// SPDX-License-Identifier: MIT

package engineenv

import (
	"math"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func TestEntityVariablesOmitUnavailableFXAndReserveNames(t *testing.T) {
	now := time.Date(2026, 7, 15, 12, 0, 0, 0, time.UTC)
	vars := Vars(Data{
		Accounts:  []domain.Account{{ID: "eur", Name: "Foreign Checking", Class: domain.ClassAsset, Type: domain.TypeChecking, Currency: "EUR", OpeningBalance: money.New(12345, "EUR"), BalanceAsOf: now}},
		Recurring: []domain.Recurring{{ID: "salary", Label: "Salary", Amount: money.New(10000, "EUR"), Cadence: domain.CadenceMonthly}},
		Rates:     currency.Rates{Base: "USD"}, Now: now,
		Molecules: []domain.Molecule{
			{Name: "account_foreign_checking_balance", Formula: "999"},
			{Name: "recurring_salary_monthly", Formula: "999"},
			{Name: "recurring_monthly_in", Formula: "999"},
			{Name: "recurring_monthly_net", Formula: "999"},
		},
	})
	for _, name := range []string{"account_foreign_checking_balance", "recurring_salary_monthly", "recurring_monthly_in", "recurring_monthly_net"} {
		if _, ok := vars[name]; ok {
			t.Errorf("unavailable %s should be omitted and reserved from molecule replacement", name)
		}
	}
	if vars["recurring_count"] != 1 {
		t.Errorf("recurring_count = %v, want known count 1", vars["recurring_count"])
	}
	if got, ok := vars["essential_monthly"]; !ok || got != 0 {
		t.Errorf("essential_monthly = %v, present %v; unrelated positive foreign income should not make it unavailable", got, ok)
	}
}

func TestRecurringVariablesCheckedFXAndCadence(t *testing.T) {
	now := time.Date(2026, 7, 15, 12, 0, 0, 0, time.UTC)
	t.Run("valid mixed FX", func(t *testing.T) {
		vars := Vars(Data{
			Recurring: []domain.Recurring{
				{ID: "pay", Label: "Pay", Amount: money.New(10000, "EUR"), Cadence: domain.CadenceMonthly},
				{ID: "rent", Label: "Rent", Amount: money.New(-5000, "USD"), Cadence: domain.CadenceMonthly},
			},
			Rates: currency.Rates{Base: "USD", Rates: map[string]float64{"EUR": 1.1}}, Now: now,
		})
		for name, want := range map[string]float64{
			"recurring_monthly_in":   110,
			"recurring_monthly_out":  50,
			"recurring_monthly_net":  60,
			"recurring_pay_monthly":  110,
			"recurring_rent_monthly": -50,
		} {
			if got := vars[name]; got != want {
				t.Errorf("%s = %v, want %v", name, got, want)
			}
		}
	})
	t.Run("weekly normalization overflow", func(t *testing.T) {
		vars := Vars(Data{
			Recurring: []domain.Recurring{{ID: "huge", Label: "Huge Salary", Amount: money.New(math.MaxInt64, "USD"), Cadence: domain.CadenceWeekly}},
			Rates:     currency.Rates{Base: "USD"}, Now: now,
		})
		for _, name := range []string{"recurring_monthly_in", "recurring_monthly_net", "recurring_huge_salary_monthly"} {
			if _, ok := vars[name]; ok {
				t.Errorf("overflowing %s should be omitted", name)
			}
		}
		if _, ok := vars["recurring_huge_salary_amount"]; !ok {
			t.Error("representable per-occurrence amount should remain available")
		}
	})
	t.Run("minimum signed monthly expense", func(t *testing.T) {
		vars := Vars(Data{
			Recurring: []domain.Recurring{{ID: "huge", Label: "Huge Expense", Amount: money.New(math.MinInt64, "USD"), Cadence: domain.CadenceMonthly}},
			Rates:     currency.Rates{Base: "USD"}, Now: now,
		})
		for _, name := range []string{"recurring_monthly_out", "recurring_monthly_net", "essential_monthly"} {
			if _, ok := vars[name]; ok {
				t.Errorf("unrepresentable %s should be omitted", name)
			}
		}
		if got, ok := vars["recurring_monthly_in"]; !ok || got != 0 {
			t.Errorf("independent known inflow = %v, present %v; want present zero", got, ok)
		}
	})
}

func TestAccountFreeUsesCheckedMinorUnitDifference(t *testing.T) {
	account := domain.Account{ID: "cash", Name: "Cash", Currency: "USD"}
	rates := currency.Rates{Base: "USD"}
	vars := map[string]float64{}
	addAccountVars(vars, Data{
		Accounts: []domain.Account{account},
		Goals:    []domain.Goal{{Allocations: []domain.GoalAllocation{{AccountID: "cash", Amount: money.New(9_007_199_254_740_992, "USD")}}}},
	}, func(v int64) float64 { return float64(v) / 100 }, rates,
		map[string]money.Money{"cash": money.New(9_007_199_254_740_993, "USD")}, nil)
	if got := vars["account_cash_free"]; got != 0.01 {
		t.Fatalf("free = %.17g, want 0.01 after subtracting in minor units", got)
	}
	if got, ok := vars["account_cash_overearmarked"]; !ok || got != 0 {
		t.Fatalf("overearmarked = %v, present %v; want known zero", got, ok)
	}
}

func TestAccountDifferenceFieldsOmitOnOverflow(t *testing.T) {
	account := domain.Account{ID: "cash", Name: "Cash", Currency: "USD"}
	vars := map[string]float64{}
	addAccountVars(vars, Data{
		Accounts: []domain.Account{account},
		Goals:    []domain.Goal{{Allocations: []domain.GoalAllocation{{AccountID: "cash", Amount: money.New(math.MinInt64, "USD")}}}},
	}, func(v int64) float64 { return float64(v) / 100 }, currency.Rates{Base: "USD"},
		map[string]money.Money{"cash": money.New(math.MaxInt64, "USD")}, nil)
	if _, ok := vars["account_cash_earmarked"]; !ok {
		t.Fatal("independently known earmarked amount should remain available")
	}
	for _, name := range []string{"account_cash_free", "account_cash_overearmarked"} {
		if _, ok := vars[name]; ok {
			t.Errorf("overflowing derived field %s should be omitted", name)
		}
	}
}

func TestGoalVariablesKeepProgressWhenBaseFXIsUnavailableAndReserveMoneyFields(t *testing.T) {
	now := time.Date(2026, 7, 15, 12, 0, 0, 0, time.UTC)
	goal := domain.Goal{ID: "g", Name: "Foreign", Kind: domain.GoalKindFinancial,
		TargetAmount: money.New(10000, "EUR"), CurrentAmount: money.New(5000, "EUR")}
	vars := Vars(Data{
		Goals: []domain.Goal{goal}, Rates: currency.Rates{Base: "USD"}, Now: now,
		Molecules: []domain.Molecule{
			{Name: "goal_foreign_target", Formula: "999"},
			{Name: "goal_foreign_saved", Formula: "999"},
			{Name: "goal_foreign_earmarked", Formula: "999"},
			{Name: "goal_foreign_coverage", Formula: "999"},
		},
	})
	if got, ok := vars["goal_foreign_progress"]; !ok || got != 50 {
		t.Errorf("progress = %v, present %v; want known native-currency 50%%", got, ok)
	}
	if got, ok := vars["goal_foreign_done"]; !ok || got != 0 {
		t.Errorf("done = %v, present %v; want known false", got, ok)
	}
	for _, name := range []string{"goal_foreign_target", "goal_foreign_saved", "goal_foreign_remaining", "goal_foreign_earmarked", "goal_foreign_coverage", "goal_foreign_covered_pct"} {
		if _, ok := vars[name]; ok {
			t.Errorf("unavailable %s should be omitted and reserved from molecule replacement", name)
		}
	}
}

func TestGoalReservationOverflowOmitsDerivedTotals(t *testing.T) {
	now := time.Date(2026, 7, 15, 12, 0, 0, 0, time.UTC)
	goal := domain.Goal{ID: "g", Name: "Overflow", TargetAmount: money.New(100, "USD"), CurrentAmount: money.New(0, "USD"),
		Allocations: []domain.GoalAllocation{{Amount: money.New(math.MaxInt64, "USD")}, {Amount: money.New(1, "USD")}}}
	vars := Vars(Data{
		Goals: []domain.Goal{goal}, Rates: currency.Rates{Base: "USD"}, Now: now,
		Molecules: []domain.Molecule{
			{Name: "earmarked_total", Formula: "999"},
			{Name: "goal_overflow_earmarked", Formula: "999"},
			{Name: "goal_overflow_coverage", Formula: "999"},
			{Name: "goal_overflow_covered_pct", Formula: "999"},
		},
	})
	for _, name := range []string{"earmarked_total", "goal_overflow_earmarked", "goal_overflow_coverage", "goal_overflow_covered_pct"} {
		if _, ok := vars[name]; ok {
			t.Errorf("unavailable %s should be omitted and reserved", name)
		}
	}
}

func TestGoalCommitmentFXFailureOmitsDependentAtoms(t *testing.T) {
	now := time.Date(2026, 7, 15, 12, 0, 0, 0, time.UTC)
	goal := domain.Goal{ID: "g", Name: "Trip", TargetAmount: money.New(12000, "EUR"), TargetDate: now.AddDate(0, 2, 0)}
	vars := Vars(Data{
		Goals: []domain.Goal{goal}, Rates: currency.Rates{Base: "USD"}, Now: now,
		Molecules: []domain.Molecule{
			{Name: "goal_needs", Formula: "999"},
			{Name: "idle_cash", Formula: "999"},
		},
	})
	for _, name := range []string{"goal_needs", "idle_cash", "idle_cash_forgone_annual"} {
		if _, ok := vars[name]; ok {
			t.Errorf("FX-dependent %s should be omitted and reserved", name)
		}
	}
}
