// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"errors"
	"testing"

	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/monthclose"
)

func TestBV6GMonthCloseUsesPerBudgetRolloverAndMonthlyEquivalent(t *testing.T) {
	items := []monthclose.Item{{BudgetID: "year", Name: "Travel", Minor: 120_000}, {BudgetID: "week", Name: "Food", Minor: 10_000}}
	budgets := []domain.Budget{
		{ID: "year", Period: domain.PeriodYearly, Rollover: true},
		{ID: "week", Period: domain.PeriodWeekly},
	}
	lines, total := monthCloseLeftovers(items, budgets, "USD")
	if len(lines) != 2 || !lines[0].rollover || lines[1].rollover {
		t.Fatalf("per-budget rollover = %#v", lines)
	}
	if lines[0].monthly == lines[0].amount {
		t.Fatalf("yearly remainder did not show a monthly equivalent: %#v", lines[0])
	}
	if total <= 0 {
		t.Fatalf("monthly equivalent total = %d, want positive", total)
	}
}

func TestBV6GMonthCloseIncomeUsesSoFarCopyWhilePeriodIsOpen(t *testing.T) {
	summary := monthclose.Summary{ExpectedIncomeMinor: 706_300, ActualIncomeMinor: 590_000}
	if got := monthCloseIncomeDeltaText(summary, "USD", false); got != "$5,900.00 received so far of $7,063.00 expected" {
		t.Fatalf("live income copy = %q", got)
	}
	if got := monthCloseIncomeDeltaText(summary, "USD", true); got != "You brought in $1,163.00 less than the plan expected — worth a look before funding next month." {
		t.Fatalf("ended income copy = %q", got)
	}
}

func TestBV6GCoverButtonTotalCountsOnlySelectedOverages(t *testing.T) {
	overs := []coverAllOver{
		{ID: "food", Shortfall: money.New(3_730, "USD")},
		{ID: "travel", Shortfall: money.New(8_000, "USD")},
	}
	amount, count := coverAllSelectedTotal(overs, map[string]string{"food": "dining", "travel": ""})
	if amount != 3_730 || count != 1 {
		t.Fatalf("selected total = (%d, %d), want (3730, 1)", amount, count)
	}
	if coverAllHasNextMonthSelection(map[string]string{"food": "dining"}) || !coverAllHasNextMonthSelection(map[string]string{"food": coverAllNextSource}) {
		t.Fatal("next-period explanation should appear only for the selected next-period option")
	}
}

func TestBV6GCoverDonorsRankByFreeRoomAfterBills(t *testing.T) {
	transportation := coverAllSource{Name: "Transportation", FreeRoom: money.New(20_000, "USD")}
	dining := coverAllSource{Name: "Dining", FreeRoom: money.New(30_000, "USD")}
	if !coverAllSourceLess(dining, transportation) || coverAllSourceLess(transportation, dining) {
		t.Fatal("donor ordering must use money left after committed bills")
	}
}

func TestBV6GRaiseRequiresPositiveParsedAmount(t *testing.T) {
	for _, tc := range []struct {
		name   string
		amount int64
		err    error
		want   bool
	}{
		{name: "empty", err: errors.New("empty")},
		{name: "zero"},
		{name: "positive", amount: 1, want: true},
		{name: "invalid", amount: 1, err: errors.New("invalid")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := budgetRaiseAmountValid(tc.amount, tc.err); got != tc.want {
				t.Fatalf("valid = %t, want %t", got, tc.want)
			}
		})
	}
}

func TestBV6GAdjustCopyUsesMinusSign(t *testing.T) {
	if got := formatAdjustPctCopy(-90); got != "−90" {
		t.Fatalf("negative percent = %q, want U+2212 minus sign", got)
	}
	if got := formatAdjustPctCopy(5.5); got != "5.5" {
		t.Fatalf("positive percent = %q, want 5.5", got)
	}
	if got := adjustScopeDescription(budgeting.AdjustThisPeriod); got != "Updates budgets in the period you are viewing." {
		t.Fatalf("single-period explanation = %q", got)
	}
	if got := adjustScopeDescription(budgeting.AdjustEveryPeriod); got != "Updates every budget period, including past periods." {
		t.Fatalf("all-period explanation = %q", got)
	}
}
