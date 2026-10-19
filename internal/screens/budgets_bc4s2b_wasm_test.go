// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/period"
)

func TestCoverAllWeeklySourceRoomCannotExceedPeriodRemainder(t *testing.T) {
	room, err := coverAllMonthlyRoom(money.New(10000, "USD"), domain.PeriodWeekly)
	if err != nil {
		t.Fatal(err)
	}
	if room.Amount != 10000 {
		t.Fatalf("weekly cover room = %d minor units, want actual period remainder 10000", room.Amount)
	}
}

func TestCoverAllCoveredAtUsesCoveredPastPeriod(t *testing.T) {
	periodStart := time.Date(2026, time.September, 28, 0, 0, 0, 0, time.UTC)
	got := coverAllCoveredAt(coverAllOver{ThisStart: periodStart})
	if !got.Equal(periodStart) {
		t.Fatalf("CoveredAt = %s, want covered period start %s", got, periodStart)
	}
}

func TestCoverAllAnchorUsesViewedCurrentLocalDate(t *testing.T) {
	pacific, err := time.LoadLocation("America/Los_Angeles")
	if err != nil {
		t.Fatal(err)
	}
	localNow := time.Date(2026, time.October, 31, 18, 0, 0, 0, pacific)
	window := period.NewWindow(period.Month, localNow, time.Monday)
	anchor := budgetViewAnchor(window, localNow)
	start, _ := budgeting.PeriodRange(domain.PeriodWeekly, anchor, time.Monday)
	want := time.Date(2026, time.October, 26, 0, 0, 0, 0, time.UTC)
	if !start.Equal(want) {
		t.Fatalf("weekly cover period starts %s, want %s for the viewed local date", start, want)
	}
}
