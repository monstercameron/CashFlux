// SPDX-License-Identifier: MIT

package appstate

import (
	"fmt"
	"time"

	"github.com/monstercameron/CashFlux/internal/budgeting"
	"github.com/monstercameron/CashFlux/internal/cardpayment"
	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
)

// This file is the read/write seam for the BG-series budget features (BG7 card-payment
// funding, BG11 unbudgeted catch-all, BG13 per-member attribution, BG16 per-period notes).
// Each method assembles the live snapshot and calls the pure logic package that owns the
// math, so the /budgets view can render these surfaces without re-deriving anything. No
// syscall/js here — the wasm layer calls in and persists (RequestPersist) after a write.

// bgRates builds the household rate table from settings (base defaults to USD).
func (a *App) bgRates() currency.Rates {
	base := a.Settings().BaseCurrency
	if base == "" {
		base = "USD"
	}
	return currency.Rates{Base: base, Rates: a.Settings().FXRates}
}

// bgMethodology is the household budgeting methodology (defaults to simple).
func (a *App) bgMethodology() budgeting.Methodology {
	return budgeting.ParseMethodology(a.Settings().BudgetMethodology)
}

// UnbudgetedThisPeriod computes the BG11 catch-all — expense in [start, end) that no budget
// covers, with its per-category breakdown — over the given window. Callers pass the same
// [start, end) the budget rows use so the catch-all and the budget bars partition the
// period's spend without overlap or gap.
func (a *App) UnbudgetedThisPeriod(start, end time.Time) (budgeting.Unbudgeted, error) {
	return budgeting.ComputeUnbudgeted(a.Budgets(), a.Categories(), a.Transactions(), start, end, a.bgRates())
}

// BudgetMemberShares returns the BG13 per-member attribution for a shared budget over
// [start, end): who spent what, summing to the budget's total. covers is the tracked
// category set (budget categories + descendants); pass nil to use the budget's own tracked
// categories. Only meaningful for a shared budget, but the helper does not gate on scope so
// callers can decide when to show the split bar.
func (a *App) BudgetMemberShares(budgetID string, start, end time.Time, covers map[string]bool) ([]budgeting.MemberShare, error) {
	var b domain.Budget
	for _, cand := range a.Budgets() {
		if cand.ID == budgetID {
			b = cand
			break
		}
	}
	if b.ID == "" {
		return nil, fmt.Errorf("appstate: budget %q not found", budgetID)
	}
	return budgeting.AttributeByMember(b, a.Transactions(), start, end, a.bgRates(), covers)
}

// CardPaymentFunding returns the BG7 payment-envelope read for every credit card, for the
// statement period containing now. It returns nil unless the household uses an envelope or
// flex methodology (the scope BG7 defines), so the surface only appears where the mechanic
// is meaningful.
func (a *App) CardPaymentFunding(now time.Time) ([]cardpayment.CardFunding, error) {
	return cardpayment.Compute(a.Accounts(), a.Transactions(), a.Budgets(), a.Categories(), a.bgRates(), now, a.bgMethodology())
}

// BudgetPeriodNote returns the BG16 journal note a budget carries for the period starting on
// periodStart (empty if none).
func (a *App) BudgetPeriodNote(budgetID string, periodStart time.Time) string {
	for _, b := range a.Budgets() {
		if b.ID == budgetID {
			return b.PeriodNote(periodStart)
		}
	}
	return ""
}

// SetBudgetPeriodNote sets (or clears, when note trims to empty) the BG16 per-period note on
// a budget and saves it. The caller persists (RequestPersist) so the note survives a refresh
// right after editing — mirroring the other durable budget edits.
func (a *App) SetBudgetPeriodNote(budgetID string, periodStart time.Time, note string) error {
	for _, b := range a.Budgets() {
		if b.ID == budgetID {
			updated := b.WithPeriodNote(periodStart, note)
			if err := a.PutBudget(updated); err != nil {
				return fmt.Errorf("appstate: set budget period note: %w", err)
			}
			a.log.Info("budget period note set", "budget", budgetID, "period", periodStart.Format("2006-01-02"))
			return nil
		}
	}
	return fmt.Errorf("appstate: budget %q not found", budgetID)
}
