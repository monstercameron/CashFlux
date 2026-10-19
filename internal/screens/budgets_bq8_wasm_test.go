// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"errors"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func TestBQ8SourcesUseViewedPeriodWhenYearlyDestinationRises(t *testing.T) {
	weekStart := time.Monday
	viewAnchor := time.Date(2026, time.October, 19, 0, 0, 0, 0, time.UTC)
	monthly := domain.Budget{ID: "monthly", Period: domain.PeriodMonthly, Limit: money.New(50_000, "USD")}
	weekly := domain.Budget{ID: "weekly", Period: domain.PeriodWeekly, Limit: money.New(50_000, "USD")}
	destination := domain.Budget{ID: "yearly", Period: domain.PeriodYearly, Limit: money.New(100_000, "USD")}
	writes, err := planFundedBudgetTopup([]domain.Budget{monthly, weekly, destination}, map[string]int64{"monthly": 3_750, "weekly": 3_750},
		"yearly", 7_500, false, viewAnchor, time.Time{}, weekStart)
	if err != nil {
		t.Fatalf("plan funded top-up: %v", err)
	}
	byID := map[string]domain.Budget{}
	for _, budget := range writes {
		byID[budget.ID] = budget
	}
	monthlyKey := budgeting.PeriodKeyWithWeekStart(monthly.Period, viewAnchor, time.Time{}, weekStart)
	if monthlyKey != "2026-10-01" {
		t.Fatalf("monthly source key = %q, want 2026-10-01", monthlyKey)
	}
	weeklyKey := budgeting.PeriodKeyWithWeekStart(weekly.Period, viewAnchor, time.Time{}, weekStart)
	if weeklyKey != "2026-10-19" {
		t.Fatalf("weekly source key = %q, want 2026-10-19", weeklyKey)
	}
	for _, source := range []struct {
		id  string
		key string
	}{{"monthly", monthlyKey}, {"weekly", weeklyKey}} {
		if got := byID[source.id].PeriodBoosts[source.key]; got != -3_750 {
			t.Errorf("%s viewed-period boost = %d, want -3750", source.id, got)
		}
		if got := byID[source.id].Limit.Amount + byID[source.id].PeriodBoosts[source.key]; got != 46_250 {
			t.Errorf("%s viewed-period cap = %d, want 46250", source.id, got)
		}
	}
	destinationKey := budgeting.PeriodKeyWithWeekStart(destination.Period, viewAnchor, time.Time{}, weekStart)
	if got := byID["yearly"].Limit.Amount + byID["yearly"].PeriodBoosts[destinationKey]; got != 107_500 {
		t.Fatalf("destination yearly cap = %d, want 107500", got)
	}
}

func TestBQ8RejectsCrossCurrencyFundedTopup(t *testing.T) {
	writes, err := planFundedBudgetTopup([]domain.Budget{
		{ID: "source", Period: domain.PeriodWeekly, Limit: money.New(50_000, "EUR")},
		{ID: "destination", Period: domain.PeriodMonthly, Limit: money.New(100_000, "USD")},
	}, map[string]int64{"source": 5_000}, "destination", 5_000, false,
		time.Date(2026, time.October, 1, 0, 0, 0, 0, time.UTC), time.Time{}, time.Monday)
	if !errors.Is(err, errBudgetTopupCurrencyMismatch) {
		t.Fatalf("cross-currency plan error = %v, want currency mismatch", err)
	}
	if writes != nil {
		t.Fatalf("cross-currency plan returned writes: %+v", writes)
	}
}
