// SPDX-License-Identifier: MIT

package engineenv

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func TestBudgetVarSlug(t *testing.T) {
	cases := map[string]string{
		"Baby & Childcare": "baby_childcare",
		"Groceries":        "groceries",
		"  Rent/Mortgage ": "rent_mortgage",
		"401(k)":           "401_k",
		"!!!":              "",
		"Dining   Out":     "dining_out",
	}
	for in, want := range cases {
		if got := budgetVarSlug(in); got != want {
			t.Errorf("budgetVarSlug(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestAddBudgetVarsSurface(t *testing.T) {
	now := time.Date(2026, 7, 15, 12, 0, 0, 0, time.UTC)
	usd := func(minor int64) money.Money { return money.New(minor, "USD") }
	budgets := []domain.Budget{
		{ID: "b1", Name: "Groceries", CategoryID: "cat-g", Period: domain.PeriodMonthly, Limit: usd(40000)},
		{ID: "b2", Name: "Rent", CategoryID: "cat-r", Period: domain.PeriodMonthly, Limit: usd(150000)},
	}
	// $250 spent in Groceries this month; nothing in Rent.
	txns := []domain.Transaction{
		{ID: "t1", CategoryID: "cat-g", Date: now, Amount: usd(-25000)},
	}
	vars := Vars(Data{
		Budgets:      budgets,
		Transactions: txns,
		Rates:        currency.Rates{Base: "USD"},
		Now:          now,
		WeekStart:    time.Sunday,
	})

	want := map[string]float64{
		"budget_groceries_limit":     400,
		"budget_groceries_spent":     250,
		"budget_groceries_remaining": 150,
		"budget_groceries_over":      0,
		"budget_groceries_percent":   62.5,
		"budget_rent_limit":          1500,
		"budget_rent_spent":          0,
		"budget_rent_remaining":      1500,
		"budget_rent_over":           0,
		"budget_rent_percent":        0,
	}
	for name, exp := range want {
		if got, ok := vars[name]; !ok {
			t.Errorf("missing surface var %q", name)
		} else if got != exp {
			t.Errorf("%s = %v, want %v", name, got, exp)
		}
	}
}

func TestAddBudgetVarsCollision(t *testing.T) {
	now := time.Date(2026, 7, 15, 12, 0, 0, 0, time.UTC)
	usd := func(minor int64) money.Money { return money.New(minor, "USD") }
	budgets := []domain.Budget{
		{ID: "b1", Name: "Fun", Period: domain.PeriodMonthly, Limit: usd(10000)},
		{ID: "b2", Name: "Fun", Period: domain.PeriodMonthly, Limit: usd(20000)},
	}
	vars := Vars(Data{Budgets: budgets, Rates: currency.Rates{Base: "USD"}, Now: now})
	if _, ok := vars["budget_fun_limit"]; !ok {
		t.Error("first Fun budget should be budget_fun_*")
	}
	if _, ok := vars["budget_fun_2_limit"]; !ok {
		t.Error("colliding Fun budget should be disambiguated to budget_fun_2_*")
	}
	if vars["budget_fun_limit"] != 100 || vars["budget_fun_2_limit"] != 200 {
		t.Errorf("collision limits wrong: %v / %v", vars["budget_fun_limit"], vars["budget_fun_2_limit"])
	}
}
