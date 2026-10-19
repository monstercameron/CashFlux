// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/period"
)

func TestBC6N2FutureIncomeUsesCurrentWallClockAnchor(t *testing.T) {
	now := time.Date(2026, time.October, 19, 12, 0, 0, 0, time.UTC)
	future := budgeting.ViewPeriod{
		Anchor:    time.Date(2027, time.February, 1, 0, 0, 0, 0, time.UTC),
		WeekStart: time.Monday,
	}
	transactions := []domain.Transaction{{
		ID: "recent-income", Date: time.Date(2026, time.September, 15, 0, 0, 0, 0, time.UTC),
		Amount: money.New(25000, "USD"),
	}}
	rates := currency.Rates{Base: "USD"}
	for _, tc := range []struct {
		name  string
		basis budgeting.ViewIncomeBasis
	}{
		{name: "averaged income", basis: budgeting.ViewIncomeBasis{Mode: budgeting.IncomeModeAll, AverageMonths: 1}},
		{name: "legacy prior month", basis: budgeting.ViewIncomeBasis{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			incomePeriod := budgetFutureIncomePeriod(future, now)
			got, err := budgeting.ResolveViewIncome(tc.basis, transactions, nil, incomePeriod, "USD", rates)
			if err != nil {
				t.Fatal(err)
			}
			if got != 25000 {
				t.Fatalf("future-period income = %d, want recent monthly income 25000", got)
			}
			if !incomePeriod.Anchor.Equal(now) {
				t.Fatalf("income anchor = %s, want current day %s", incomePeriod.Anchor, now)
			}
			if !future.Anchor.After(incomePeriod.Anchor) {
				t.Fatalf("test did not keep future budget anchor separate: %s", future.Anchor)
			}
		})
	}
}

func TestBC6N2RecurringDateUsesWallClockDay(t *testing.T) {
	local := time.FixedZone("UTC-4", -4*60*60)
	now := time.Date(2026, time.October, 18, 23, 30, 0, 0, local)
	nextDue := time.Date(2026, time.October, 19, 0, 0, 0, 0, time.UTC)
	periodStart := time.Date(2026, time.October, 1, 0, 0, 0, 0, time.UTC)
	periodEnd := time.Date(2026, time.November, 1, 0, 0, 0, 0, time.UTC)
	if got := recurringDateState(nextDue, now, periodStart, periodEnd); got != budgeting.RecurDueInPeriod {
		t.Fatalf("date state = %q, want due-in-period on the user's October 18 wall-clock day", got)
	}
}

func TestBC6N2FutureWindowUsesWallClockDay(t *testing.T) {
	local := time.FixedZone("UTC-4", -4*60*60)
	now := time.Date(2026, time.October, 18, 23, 30, 0, 0, local)
	start := time.Date(2026, time.October, 19, 0, 0, 0, 0, time.UTC)
	window := period.NewWindow(period.Week, start, time.Monday)
	if !isFuturePeriod(window, now) {
		t.Fatal("window starting tomorrow's wall-clock day was not treated as future")
	}
}

func TestBudgetHistoricalWindowUsesWallClockDay(t *testing.T) {
	local := time.FixedZone("UTC-4", -4*60*60)
	now := time.Date(2026, time.October, 18, 23, 30, 0, 0, local)
	end := time.Date(2026, time.October, 19, 0, 0, 0, 0, time.UTC)
	if budgetWindowIsHistorical(end, now) {
		t.Fatal("window ending at tomorrow's wall-clock boundary was marked historical")
	}
}
