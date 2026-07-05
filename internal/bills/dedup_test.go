// SPDX-License-Identifier: MIT

package bills

import (
	"testing"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
)

// TestUpcomingAllDedupesLiabilityRecurring pins the v1.0 fix: a liability's own
// statement bill and a recurring flow modelling the SAME payment (matching
// currency, amount, and due day) must appear once, not twice — the double-count
// that inflated the headline "total due" and "per year".
func TestUpcomingAllDedupesLiabilityRecurring(t *testing.T) {
	now := time.Date(2026, 7, 5, 0, 0, 0, 0, time.UTC)
	accounts := []domain.Account{{
		ID: "acct-carloan", Class: domain.ClassLiability, Type: domain.TypeLoan,
		Currency: "USD", DueDayOfMonth: 15, MinPayment: money.New(-62000, "USD"),
	}}
	recurring := []domain.Recurring{
		// Same obligation as the loan's statement bill → must be deduped.
		{ID: "rec-carpay", Label: "Car payment", Amount: money.New(-62000, "USD"), Cadence: domain.CadenceMonthly, NextDue: time.Date(2026, 7, 15, 0, 0, 0, 0, time.UTC), AccountID: "acct-checking"},
		// A genuine standalone recurring (HOA) → must be kept.
		{ID: "rec-hoa", Label: "HOA dues", Amount: money.New(-38000, "USD"), Cadence: domain.CadenceMonthly, NextDue: time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC), AccountID: "acct-checking"},
	}

	bills := UpcomingAll(accounts, recurring, now)
	carCount := 0
	hoaCount := 0
	for _, b := range bills {
		if b.Amount.Amount == 62000 {
			carCount++
		}
		if b.Amount.Amount == 38000 {
			hoaCount++
		}
	}
	if carCount != 1 {
		t.Errorf("car payment should appear once (liability bill), got %d", carCount)
	}
	if hoaCount != 1 {
		t.Errorf("standalone HOA recurring should be kept, got %d", hoaCount)
	}

	// AnnualAmounts must not double the loan payment either.
	annual := AnnualAmounts(accounts, recurring)
	var carYr int
	for _, m := range annual {
		if m.Amount == 62000*12 {
			carYr++
		}
	}
	if carYr != 1 {
		t.Errorf("annual car payment should be counted once, got %d entries of %d", carYr, 62000*12)
	}
}
