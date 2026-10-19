// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"math"
	"testing"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func TestBudgetRecurringCoverageHonorsMemberLens(t *testing.T) {
	budgets := []domain.Budget{
		{ID: "alice-food", Scope: domain.ScopeIndividual, OwnerID: "alice", CategoryID: "food"},
		{ID: "shared-home", Scope: domain.ScopeShared, CategoryID: "home"},
	}
	categories := []domain.Category{{ID: "food"}, {ID: "home"}}
	got := recurringBudgetCoverage(budgets, categories, "bob")
	if got["food"] {
		t.Fatal("another member's budget must not mark this member's bill covered")
	}
	if !got["home"] {
		t.Fatal("shared budgets should remain in the coverage set")
	}
}

func TestBudgetRecurringMonthlyBaseExcludesUnavailableBills(t *testing.T) {
	r := domain.Recurring{Amount: money.New(-5000, "EUR"), Cadence: domain.CadenceMonthly}
	if _, err := recurringMonthlyBase(r, "USD", currency.Rates{Base: "USD"}); err == nil {
		t.Fatal("missing FX rate should make the bill unavailable")
	}
}

func TestBudgetRecurringMonthlyEquivalentChecksOverflow(t *testing.T) {
	r := domain.Recurring{Amount: money.New(math.MinInt64, "USD"), Cadence: domain.CadenceWeekly}
	if _, err := recurringMonthlyEquivalentChecked(r); err == nil {
		t.Fatal("weekly monthly-equivalent overflow must be reported")
	}
}
