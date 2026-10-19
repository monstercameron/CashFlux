// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
)

func TestMonthCloseCopyWritesNextPeriodOnce(t *testing.T) {
	app := guardTestApp(t)
	budget := quickFillFixtureBudget()
	budget.Period = domain.PeriodMonthly
	current := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	next := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	budget = budget.WithPeriodBoost(current, 5_000)
	if err := app.PutBudgetAt(budget, current); err != nil {
		t.Fatal(err)
	}
	anchor := time.Date(2026, 10, 31, 12, 0, 0, 0, time.UTC)
	excluded := func() string { return "" }
	if err := applyMonthCloseCopy(app, app.Budgets(), anchor, time.Sunday, time.Time{}, excluded); err != nil {
		t.Fatalf("first apply: %v", err)
	}
	got := app.Budgets()[0]
	if got.PeriodBoost(next) != 5_000 || got.PeriodBoost(current) != 5_000 {
		t.Fatalf("first apply boosts = %v, want current and next both 5000", got.PeriodBoosts)
	}
	if err := applyMonthCloseCopy(app, app.Budgets(), anchor, time.Sunday, time.Time{}, excluded); err != nil {
		t.Fatalf("repeat apply: %v", err)
	}
	got = app.Budgets()[0]
	if got.PeriodBoost(next) != 5_000 {
		t.Fatalf("repeat apply stacked the target boost: %v", got.PeriodBoosts)
	}
}

func TestMonthCloseEndCheckUsesWallClockAcrossTimeZones(t *testing.T) {
	monthEnd := time.Date(2026, time.November, 1, 0, 0, 0, 0, time.UTC)
	pacific := time.Date(2026, time.October, 31, 18, 0, 0, 0, time.FixedZone("Pacific", -7*60*60))
	tokyo := time.Date(2026, time.November, 1, 5, 0, 0, 0, time.FixedZone("Tokyo", 9*60*60))
	if monthClosePeriodEnded(monthEnd, pacific) {
		t.Fatal("Pacific evening on October 31 must still be inside the October month-close window")
	}
	if !monthClosePeriodEnded(monthEnd, tokyo) {
		t.Fatal("Tokyo morning on November 1 must be after the October month-close window")
	}
}
