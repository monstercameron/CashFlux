// SPDX-License-Identifier: MIT

package safespend

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

func identity(amount int64, _ string) int64 { return amount }

var testRates = currency.Rates{Base: "USD", Rates: map[string]float64{"EUR": 1.1}}

func TestBillsDueBefore(t *testing.T) {
	now := time.Date(2026, 1, 10, 0, 0, 0, 0, time.UTC)
	cutoff := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)

	accounts := []domain.Account{
		{ID: "a1", Name: "Card", Class: domain.ClassLiability, Currency: "USD",
			DueDayOfMonth: 15, MinPayment: money.New(5000, "USD")},
		{ID: "a2", Name: "Late", Class: domain.ClassLiability, Currency: "USD",
			DueDayOfMonth: 5, MinPayment: money.New(3000, "USD")}, // due Feb 5 — after cutoff
	}

	got := BillsDueBefore(accounts, nil, now, cutoff, identity)
	// Only a1 (due Jan 15) should be included; a2 is due Feb 5.
	if got != 5000 {
		t.Errorf("BillsDueBefore = %d, want 5000", got)
	}
}

func TestBillsDueBefore_NoBills(t *testing.T) {
	now := time.Date(2026, 1, 10, 0, 0, 0, 0, time.UTC)
	cutoff := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)
	got := BillsDueBefore(nil, nil, now, cutoff, identity)
	if got != 0 {
		t.Errorf("BillsDueBefore empty = %d, want 0", got)
	}
}

func TestGoalContributionsProrated(t *testing.T) {
	target := time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC)
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	gs := []domain.Goal{
		{
			ID:            "g1",
			Name:          "Vacation",
			TargetAmount:  money.New(120000, "USD"), // $1200
			CurrentAmount: money.New(0, "USD"),
			TargetDate:    target,
		},
		{
			ID:           "g2",
			Name:         "Archived",
			Archived:     true,
			TargetAmount: money.New(50000, "USD"),
		},
	}

	got := GoalContributionsProrated(gs, now, identity)
	// 12 months to target, $1200 remaining → $100/month = 10000 minor units.
	if got != 10000 {
		t.Errorf("GoalContributionsProrated = %d, want 10000", got)
	}
}

func TestGoalContributions_Archived(t *testing.T) {
	gs := []domain.Goal{
		{ID: "g1", Archived: true, TargetAmount: money.New(10000, "USD")},
	}
	got := GoalContributionsProrated(gs, time.Now(), identity)
	if got != 0 {
		t.Errorf("archived goal should contribute 0, got %d", got)
	}
}

func TestToBaseFunc(t *testing.T) {
	conv := ToBaseFunc(testRates)
	// 100 EUR at 1.1 USD/EUR = 110 USD minor units (same precision).
	got := conv(100, "EUR")
	if got != 110 {
		t.Errorf("ToBaseFunc EUR→USD 100 = %d, want 110", got)
	}
	// same currency is identity.
	if got2 := conv(200, "USD"); got2 != 200 {
		t.Errorf("ToBaseFunc USD→USD 200 = %d, want 200", got2)
	}
}
