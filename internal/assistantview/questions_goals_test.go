// SPDX-License-Identifier: MIT

package assistantview

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/goals"
	"github.com/monstercameron/CashFlux/internal/money"
)

func TestQuestionsSkipConfirmedCompleteGoalsAndKeepUnknownFX(t *testing.T) {
	now := time.Date(2026, 10, 20, 0, 0, 0, 0, time.UTC)
	input := []domain.Goal{
		{ID: "funded", Name: "Funded", TargetAmount: money.New(1000, "USD"), CurrentAmount: money.New(100, "USD"), TargetDate: now.AddDate(0, 0, 1),
			Allocations: []domain.GoalAllocation{{Amount: money.New(450, "EUR")}}},
		{ID: "checklist", Name: "Done checklist", Kind: domain.GoalKindChecklist, TargetDate: now.AddDate(0, 0, 1)},
		{ID: "unknown", Name: "Unknown FX", TargetAmount: money.New(1000, "USD"), CurrentAmount: money.New(1000, "JPY"), TargetDate: now.AddDate(0, 0, 1)},
		{ID: "active", Name: "Active", TargetAmount: money.New(1000, "USD"), CurrentAmount: money.Zero("USD"), TargetDate: now.AddDate(0, 0, 2)},
	}
	tasks := goals.BuildTaskIndex([]domain.Task{{ID: "done", RelatedType: domain.RelatedGoal, RelatedID: "checklist", Status: domain.StatusDone}})
	rates := currency.Rates{Base: "USD", Rates: map[string]float64{"EUR": 2}}
	got := Questions(nil, nil, nil, input, rates, now, now.AddDate(0, 1, 0), now, tasks)
	if got.UpcomingGoal != "Unknown FX" {
		t.Fatalf("completed goals stayed in starters or unknown coverage was skipped: %+v", got)
	}
	rates.Rates["JPY"] = 1
	got = Questions(nil, nil, nil, input, rates, now, now.AddDate(0, 1, 0), now, tasks)
	// JPY has zero decimals: 1000 JPY at 1 USD/JPY covers this $10 target.
	if got.UpcomingGoal != "Active" || input[0].Allocations[0].Amount != money.New(450, "EUR") || input[2].CurrentAmount.Currency != "JPY" {
		t.Fatalf("FX resolution did not update starter eligibility without rewriting goals: %+v", got)
	}
}
