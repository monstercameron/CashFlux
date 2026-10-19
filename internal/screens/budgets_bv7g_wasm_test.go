// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"testing"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/monthclose"
)

func TestBV7GCoverDonorShowsMonthlyEquivalentAndRanksByFreeRoom(t *testing.T) {
	weekly, err := coverAllMonthlyEquivalent(money.New(2500, "USD"), domain.PeriodWeekly)
	if err != nil {
		t.Fatal(err)
	}
	if weekly.Amount != 10833 {
		t.Fatalf("weekly monthly equivalent = %d minor units, want 10833", weekly.Amount)
	}
	actualRoom, err := coverAllMonthlyRoom(money.New(2500, "USD"), domain.PeriodWeekly)
	if err != nil {
		t.Fatal(err)
	}
	if actualRoom.Amount != 2500 {
		t.Fatalf("weekly movable amount = %d minor units, want actual period balance 2500", actualRoom.Amount)
	}
	if !coverAllSourceLess(coverAllSource{Name: "Groceries", FreeRoom: money.New(20000, "USD")},
		coverAllSource{Name: "Dining", FreeRoom: money.New(10000, "USD")}) {
		t.Fatal("donor order did not prefer the larger room after committed bills")
	}
}

func TestBV7GMonthCloseLeftoversShowMonthlyEquivalentAndCarryOutcome(t *testing.T) {
	weekly := quickFillFixtureBudget()
	weekly.ID = "weekly"
	weekly.Name = "Entertainment"
	weekly.Period = domain.PeriodWeekly
	weekly.Rollover = true
	monthly := quickFillFixtureBudget()
	monthly.ID = "monthly"
	monthly.Name = "Dining"
	monthly.Period = domain.PeriodMonthly
	items := []monthclose.Item{
		{BudgetID: weekly.ID, Name: weekly.Name, Minor: 2500},
		{BudgetID: monthly.ID, Name: monthly.Name, Minor: 2500},
	}
	lines, total := monthCloseLeftovers(items, []domain.Budget{weekly, monthly}, "USD")
	if len(lines) != 2 {
		t.Fatalf("leftover rows = %d, want 2", len(lines))
	}
	if lines[0].outcome != "bv7g.monthclose.carries" || lines[1].outcome != "bv7g.monthclose.resets" {
		t.Fatalf("next-period outcomes = %q, %q", lines[0].outcome, lines[1].outcome)
	}
	if lines[0].monthly != "$108.33" || lines[1].monthly != "$25.00" || total != 13333 {
		t.Fatalf("monthly equivalents = %q, %q; total %d, want $108.33, $25.00, 13333", lines[0].monthly, lines[1].monthly, total)
	}
}
