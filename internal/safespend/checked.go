// SPDX-License-Identifier: MIT

package safespend

import (
	"fmt"
	"time"

	"github.com/monstercameron/CashFlux/internal/bills"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/goals"
	"github.com/monstercameron/CashFlux/internal/money"
)

// BillsDueBeforeChecked is BillsDueBefore with explicit magnitude, conversion
// and aggregation errors. UpcomingAll remains the authority for bill eligibility,
// deduplication and due-date rules.
func BillsDueBeforeChecked(accounts []domain.Account, recurring []domain.Recurring, now, cutoff time.Time, rates currency.Rates) (int64, error) {
	for _, a := range accounts {
		if a.Archived || a.Class != domain.ClassLiability || a.DueDayOfMonth <= 0 || a.MinPayment.Amount == 0 {
			continue
		}
		if _, err := a.MinPayment.AbsChecked(); err != nil {
			return 0, fmt.Errorf("safespend: account %s bill magnitude: %w", a.ID, err)
		}
	}
	for _, r := range recurring {
		if r.Active() && r.Amount.IsNegative() {
			if _, err := r.Amount.AbsChecked(); err != nil {
				return 0, fmt.Errorf("safespend: recurring %s bill magnitude: %w", r.ID, err)
			}
		}
	}
	var total int64
	for _, bill := range bills.UpcomingAll(accounts, recurring, now) {
		if !bill.DueDate.Before(cutoff) {
			continue
		}
		converted, err := rates.Convert(bill.Amount, rates.Base)
		if err != nil {
			return 0, fmt.Errorf("safespend: bill %s FX: %w", bill.AccountID, err)
		}
		total, err = money.Add(total, converted.Amount)
		if err != nil {
			return 0, fmt.Errorf("safespend: bill total: %w", err)
		}
	}
	return total, nil
}

// GoalContributionsProratedChecked is GoalContributionsProrated with explicit
// conversion and aggregation errors. It uses the same MonthlyNeeded policy,
// including pause, target-date and coverage rules.
func GoalContributionsProratedChecked(gs []domain.Goal, now time.Time, rates currency.Rates) (int64, error) {
	var total int64
	for _, g := range gs {
		if g.Archived {
			continue
		}
		needed, ok, err := goals.MonthlyNeeded(g, now)
		if err != nil {
			return 0, fmt.Errorf("safespend: goal %s monthly contribution: %w", g.ID, err)
		}
		if !ok {
			continue
		}
		converted, err := rates.Convert(needed, rates.Base)
		if err != nil {
			return 0, fmt.Errorf("safespend: goal %s contribution FX: %w", g.ID, err)
		}
		total, err = money.Add(total, converted.Amount)
		if err != nil {
			return 0, fmt.Errorf("safespend: goal contribution total: %w", err)
		}
	}
	return total, nil
}
