// SPDX-License-Identifier: MIT

package budgetview

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func TestAnnualProjectionUsesCanonicalGoalCoverageAndUnknownErrors(t *testing.T) {
	now := time.Date(2026, 10, 20, 0, 0, 0, 0, time.UTC)
	build := BuildAnnual([]domain.Budget{budget("food", 1000)}, nil, []domain.Category{{ID: "c", Name: "Food", Kind: domain.KindExpense}}, 2026, usd(), time.Monday, now)
	goal := domain.Goal{ID: "goal", CategoryID: "c", TargetAmount: money.New(1000, "USD"), CurrentAmount: money.New(100, "USD"), TargetDate: time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC),
		Allocations: []domain.GoalAllocation{{Amount: money.New(400, "EUR")}}}
	projection := ProjectAnnual(build.Grid, build.Covers, nil, []domain.Goal{goal}, now, currency.Rates{Base: "USD", Rates: map[string]float64{"EUR": 2}})
	if projection.Err != nil || projection.Totals[10] != 100 || projection.PerBudget["food"][10] != 100 {
		t.Fatalf("annual grid lost converted goal reservations: %+v", projection)
	}
	unknown := ProjectAnnual(build.Grid, build.Covers, nil, []domain.Goal{goal}, now, usd())
	if unknown.Err == nil {
		t.Fatal("annual grid presented missing goal FX as a zero spending plan")
	}
}
