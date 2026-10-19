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

func TestBQ8WeeklySourceLowersOwnWeekWhenMonthlyDestinationRises(t *testing.T) {
	weekStart := time.Monday
	monthStart := time.Date(2026, time.October, 1, 0, 0, 0, 0, time.UTC)
	source := domain.Budget{ID: "weekly", Period: domain.PeriodWeekly, Limit: money.New(50_000, "USD")}
	destination := domain.Budget{ID: "monthly", Period: domain.PeriodMonthly, Limit: money.New(100_000, "USD")}
	writes, err := planFundedBudgetTopup([]domain.Budget{source, destination}, map[string]int64{"weekly": 7_500},
		"monthly", 7_500, false, monthStart, time.Time{}, weekStart)
	if err != nil {
		t.Fatalf("plan funded top-up: %v", err)
	}
	byID := map[string]domain.Budget{}
	for _, budget := range writes {
		byID[budget.ID] = budget
	}
	sourceKey := budgeting.PeriodKeyWithWeekStart(source.Period, monthStart, time.Time{}, weekStart)
	if sourceKey != "2026-09-28" {
		t.Fatalf("weekly source key = %q, want 2026-09-28", sourceKey)
	}
	if got := byID["weekly"].PeriodBoosts[sourceKey]; got != -7_500 {
		t.Fatalf("source own-week boost = %d, want -7500", got)
	}
	if _, wronglyKeyed := byID["weekly"].PeriodBoosts["2026-10-01"]; wronglyKeyed {
		t.Fatal("source was written under the monthly destination key")
	}
	if got := source.Limit.Amount + byID["weekly"].PeriodBoosts[sourceKey]; got != 42_500 {
		t.Fatalf("source current-week cap = %d, want 42500", got)
	}
	destinationKey := budgeting.PeriodKeyWithWeekStart(destination.Period, monthStart, time.Time{}, weekStart)
	if got := byID["monthly"].Limit.Amount + byID["monthly"].PeriodBoosts[destinationKey]; got != 107_500 {
		t.Fatalf("destination month cap = %d, want 107500", got)
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
