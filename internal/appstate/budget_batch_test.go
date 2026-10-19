// SPDX-License-Identifier: MIT

package appstate

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
)

func TestPutBudgetsAtomicUpdatesTogether(t *testing.T) {
	a := newApp(t, false)
	for _, budget := range []domain.Budget{
		sharedBudget("source", "Source", "cat-source", 50_000),
		sharedBudget("destination", "Destination", "cat-destination", 100_000),
	} {
		if err := a.PutBudget(budget); err != nil {
			t.Fatalf("PutBudget(%s): %v", budget.ID, err)
		}
	}
	byID := make(map[string]domain.Budget)
	for _, budget := range a.Budgets() {
		byID[budget.ID] = budget
	}
	source, destination := byID["source"], byID["destination"]
	source = source.WithPeriodBoost(moneyDate("2026-09-28"), -7_500)
	destination = destination.WithPeriodBoost(moneyDate("2026-10-01"), 7_500)
	if err := a.PutBudgetsAtomic([]domain.Budget{source, destination}); err != nil {
		t.Fatalf("PutBudgetsAtomic: %v", err)
	}
	got := make(map[string]domain.Budget)
	for _, budget := range a.Budgets() {
		got[budget.ID] = budget
	}
	if got["source"].PeriodBoosts["2026-09-28"] != -7_500 || got["destination"].PeriodBoosts["2026-10-01"] != 7_500 {
		t.Fatalf("saved budgets = %+v, want both balanced period boosts", got)
	}
}

func moneyDate(value string) time.Time {
	parsed, err := time.Parse("2006-01-02", value)
	if err != nil {
		panic(err)
	}
	return parsed
}
