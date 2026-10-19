// SPDX-License-Identifier: MIT

//go:build js && wasm

package screens

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/goalfunded"
	"github.com/monstercameron/CashFlux/internal/money"
	"github.com/monstercameron/CashFlux/internal/prefs"
)

func TestBudgetWhyOverUsesEvaluatedCardLimit(t *testing.T) {
	status := budgeting.Status{
		Budget: domain.Budget{ID: "dining", Limit: money.New(30000, "USD")},
		Limit:  money.New(20000, "USD"), Spent: money.New(25000, "USD"), Remaining: money.New(-5000, "USD"),
		Projection: budgeting.Pace{Elapsed: .8, Projected: money.New(31250, "USD")},
	}
	result, ok := explainWhyOverStatus(status, 5000, 8, 18000, 6)
	if !ok || result.OverMinor != 5000 {
		t.Fatalf("why-over result = %+v, ok=%v; want $50 over evaluated $200 cap", result, ok)
	}
}

func TestBudgetRowSavingCopyAndMissingRateSentinel(t *testing.T) {
	status := budgeting.Status{Budget: domain.Budget{Direction: domain.DirectionSave}, Remaining: money.New(40000, "USD")}
	if got := budgetRowRemainPhrase(status); got != "short by $400.00" {
		t.Fatalf("saving remainder = %q", got)
	}
	status.Remaining = money.Zero("USD")
	if got := budgetRowRemainPhrase(status); got != "target met" {
		t.Fatalf("met saving remainder = %q", got)
	}
	if !budgetErrorHasUnknownRate(fmt.Errorf("budget conversion: %w", currency.ErrUnknownRate)) {
		t.Fatal("wrapped unknown-rate error was not recognized")
	}
	if budgetErrorHasUnknownRate(errors.New("exchange rate unavailable")) {
		t.Fatal("error text alone must not trigger the rate-fix action")
	}
}

func TestBudgetStartPickerParsesAndAnchorsMonth(t *testing.T) {
	got, err := parseBudgetStartPeriod("2026-03")
	if err != nil || got != time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC) {
		t.Fatalf("parsed start = %v, %v; want first of March 2026 UTC", got, err)
	}
	if _, err := parseBudgetStartPeriod("March 2026"); err == nil {
		t.Fatal("invalid month text must not save a rollover start")
	}
}

func TestBudgetRolloverMonthLabelUsesCatalog(t *testing.T) {
	got := budgetRolloverMonthLabel(time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC))
	if got != "Oct 2026" {
		t.Fatalf("rollover month label = %q; want translated short month + year", got)
	}
}

func TestBudgetMissingRateUsesSentinel(t *testing.T) {
	if !budgetErrorHasUnknownRate(fmt.Errorf("wrapped: %w", currency.ErrUnknownRate)) {
		t.Fatal("unknown-rate sentinel was not recognized")
	}
	if budgetErrorHasUnknownRate(errors.New("exchange rate problem from another layer")) {
		t.Fatal("message text must not classify an unrelated error as a missing rate")
	}
}

func TestBudgetRemainingPerDayCountsCalendarDays(t *testing.T) {
	status := budgeting.Status{
		Budget:       domain.Budget{Period: domain.PeriodMonthly},
		PeriodStatus: budgeting.PeriodActive,
		Remaining:    money.New(10000, "USD"),
	}
	anchor := time.Date(2026, 10, 30, 0, 0, 0, 0, time.UTC)
	now := time.Date(2026, 10, 30, 10, 0, 0, 0, time.UTC)
	got, ok := budgetRowRestPerDay(status, anchor, prefs.Prefs{}, now)
	if !ok || got.Amount != 5000 {
		t.Fatalf("rest per day = %+v, ok=%v; want $50 across Oct 30 and 31", got, ok)
	}
}

func TestBudgetGoalFundingAllocationMemoIsPerRevisionAndWindow(t *testing.T) {
	start := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	end := start.AddDate(0, 1, 0)
	calls := 0
	calculate := func() []goalfunded.BudgetFunding {
		calls++
		return []goalfunded.BudgetFunding{{GoalID: "g", BudgetID: "b", AmountMinor: 500}}
	}
	one := memoBudgetGoalFunding(nil, 781234, start, end, calculate)
	two := memoBudgetGoalFunding(nil, 781234, start, end, calculate)
	if calls != 1 || len(one) != 1 || len(two) != 1 || two[0].AmountMinor != 500 {
		t.Fatalf("memo calls=%d first=%+v second=%+v", calls, one, two)
	}
}

func TestBudgetGoalFundingUsesLinkedNettedLedger(t *testing.T) {
	transactions := []domain.Transaction{
		{ID: "purchase", Amount: money.New(-12000, "USD"), Date: time.Date(2026, 3, 10, 0, 0, 0, 0, time.UTC)},
		{ID: "refund", Amount: money.New(4000, "USD"), Date: time.Date(2026, 4, 5, 0, 0, 0, 0, time.UTC)},
	}
	links := []domain.TxnLink{{ID: "refund-pair", Kind: domain.TxnLinkRefundPair, TxnIDs: []string{"purchase", "refund"}, Amount: money.New(4000, "USD")}}
	got := budgetGoalFundingTransactions(transactions, links)
	if len(got) != 2 || got[0].Amount.Amount != -8000 || got[1].Amount.Amount != 0 {
		t.Fatalf("goal funding ledger = %+v; want March purchase net of refund and zero refund row", got)
	}
}

func TestBudgetDriverMemoIsPerRevisionAndWindow(t *testing.T) {
	start := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	end := start.AddDate(0, 1, 0)
	calls := 0
	calculate := func() ([]budgeting.Driver, string) {
		calls++
		return []budgeting.Driver{{TxnID: "t", Amount: 1200, Currency: "EUR"}}, "USD"
	}
	first, base := memoBudgetDrivers(nil, 782001, "b-drivers", start, end, calculate)
	second, secondBase := memoBudgetDrivers(nil, 782001, "b-drivers", start, end, calculate)
	if calls != 1 || base != "USD" || secondBase != "USD" || len(first) != 1 || len(second) != 1 || second[0].Currency != "EUR" {
		t.Fatalf("driver memo calls=%d first=%+v second=%+v base=%q/%q", calls, first, second, base, secondBase)
	}
}
