// SPDX-License-Identifier: MIT

package safespend

import (
	"math"
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func TestGoalContributionsProratedCheckedMatchesLegacyForKnownInputs(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	goals := []domain.Goal{
		{ID: "usd", TargetAmount: money.New(12000, "USD"), TargetDate: now.AddDate(0, 1, 0)},
		{ID: "eur", TargetAmount: money.New(10000, "EUR"), TargetDate: now.AddDate(0, 1, 0)},
		{ID: "archived", Archived: true, TargetAmount: money.New(90000, "EUR"), TargetDate: now.AddDate(0, 1, 0)},
	}
	rates := currency.Rates{Base: "USD", Rates: map[string]float64{"EUR": 1.1}}
	got, err := GoalContributionsProratedChecked(goals, now, rates)
	if err != nil {
		t.Fatal(err)
	}
	want := GoalContributionsProrated(goals, now, ToBaseFunc(rates))
	if got != want {
		t.Fatalf("checked = %d, legacy = %d", got, want)
	}
}

func TestGoalContributionsProratedCheckedRejectsUnknownFXAndOverflow(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	base := func(id, code string, amount int64) domain.Goal {
		return domain.Goal{ID: id, TargetAmount: money.New(amount, code), TargetDate: now.AddDate(0, 1, 0)}
	}
	t.Run("unknown FX", func(t *testing.T) {
		if _, err := GoalContributionsProratedChecked([]domain.Goal{base("foreign", "EUR", 10000)}, now, currency.Rates{Base: "USD"}); err == nil {
			t.Fatal("unknown EUR rate should make goal commitments unavailable")
		}
	})
	t.Run("sum overflow", func(t *testing.T) {
		goals := []domain.Goal{base("first", "USD", math.MaxInt64), base("second", "USD", 1)}
		if _, err := GoalContributionsProratedChecked(goals, now, currency.Rates{Base: "USD"}); err == nil {
			t.Fatal("overflowing goal commitment sum should fail")
		}
	})
}

func TestBillsDueBeforeCheckedRejectsUnknownFXAndOverflow(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	cutoff := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)
	t.Run("unknown FX", func(t *testing.T) {
		rows := []domain.Recurring{{ID: "eur", Label: "Euro bill", Amount: money.New(-10000, "EUR"), Cadence: domain.CadenceMonthly, NextDue: now.AddDate(0, 0, 2)}}
		if _, err := BillsDueBeforeChecked(nil, rows, now, cutoff, currency.Rates{Base: "USD"}); err == nil {
			t.Fatal("unknown EUR rate should make due-bill total unavailable")
		}
	})
	t.Run("sum overflow", func(t *testing.T) {
		accounts := []domain.Account{
			{ID: "one", Class: domain.ClassLiability, Currency: "USD", DueDayOfMonth: 15, MinPayment: money.New(math.MaxInt64, "USD")},
			{ID: "two", Class: domain.ClassLiability, Currency: "USD", DueDayOfMonth: 16, MinPayment: money.New(1, "USD")},
		}
		if _, err := BillsDueBeforeChecked(accounts, nil, now, cutoff, currency.Rates{Base: "USD"}); err == nil {
			t.Fatal("overflowing bill total should fail")
		}
	})
}
