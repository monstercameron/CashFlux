// SPDX-License-Identifier: MIT

package budgeting

import (
	"math"

	"github.com/monstercameron/CashFlux/internal/domain"
)

// AdjustAll's accepted percentage bounds. A lower past −90% would leave budgets
// at a token cent; a raise past +500% is far likelier to be a typo (500 meant as
// "$500") than an intent, and either way the user can apply twice.
const (
	AdjustMinPct = -90.0
	AdjustMaxPct = 500.0
	// AdjustLargePct is the magnitude past which a bulk change is treated as
	// consequential and asked about explicitly rather than just previewed.
	AdjustLargePct = 25.0
)

// AdjustLine is one budget's before/after under a bulk adjustment.
type AdjustLine struct {
	Budget domain.Budget
	Before int64 // minor units, the budget's own currency
	After  int64
}

// Delta is the change to this budget's limit; negative means a reduction.
func (l AdjustLine) Delta() int64 { return l.After - l.Before }

// AdjustPreview is everything the "Adjust all" form must show BEFORE it writes:
// which budgets change, what each becomes, and what the household's total
// budgeted amount goes from and to.
//
// It exists because the control used to be a bare prompt asking for a percentage
// and a confirm sentence quoting it back (C592). Neither said how many budgets
// were in scope, what the total was, or what it would become — so "lower
// everything by 10%" was a number typed into the dark.
type AdjustPreview struct {
	Lines       []AdjustLine
	TotalBefore int64
	TotalAfter  int64
	// Currency is the shared currency of the affected budgets, or "" when they
	// disagree — in which case the totals are not meaningful and the form shows
	// per-budget lines only.
	Currency string
	// MixedCurrency reports that the affected budgets are not all in one
	// currency, so TotalBefore/TotalAfter must not be presented as money.
	MixedCurrency bool
}

// Count is how many budgets the adjustment touches.
func (p AdjustPreview) Count() int { return len(p.Lines) }

// TotalDelta is the change to the household's total budgeted amount.
func (p AdjustPreview) TotalDelta() int64 { return p.TotalAfter - p.TotalBefore }

// ValidAdjustPct reports whether pct is an adjustment the form may apply: a real
// change, within bounds. Zero is rejected as a no-op rather than silently
// rewriting every budget to itself.
func ValidAdjustPct(pct float64) bool {
	return pct != 0 && pct >= AdjustMinPct && pct <= AdjustMaxPct && !math.IsNaN(pct)
}

// IsLargeAdjust reports whether an adjustment is consequential enough to ask
// about explicitly: any reduction (money coming out of every plan at once) or a
// change of more than AdjustLargePct in either direction.
func IsLargeAdjust(pct float64) bool { return pct < 0 || math.Abs(pct) > AdjustLargePct }

// AdjustAllPreview computes the before/after for raising or lowering every given
// budget's limit by pct percent. Budgets with a non-positive limit are skipped —
// there is nothing to scale — and a reduction floors at one minor unit, because a
// budget with a non-positive limit fails validation: a bulk lower may shrink a
// budget, never delete it.
//
// Pure arithmetic over integer minor units, rounded half-away-from-zero, so the
// preview the user reads and the write that follows cannot differ.
func AdjustAllPreview(budgets []domain.Budget, pct float64) AdjustPreview {
	var p AdjustPreview
	for _, b := range budgets {
		if b.Limit.Amount <= 0 {
			continue
		}
		after := AdjustedLimit(b.Limit.Amount, pct)
		p.Lines = append(p.Lines, AdjustLine{Budget: b, Before: b.Limit.Amount, After: after})
		p.TotalBefore += b.Limit.Amount
		p.TotalAfter += after
		switch {
		case p.Currency == "":
			p.Currency = b.Limit.Currency
		case p.Currency != b.Limit.Currency:
			p.MixedCurrency = true
		}
	}
	if p.MixedCurrency {
		p.Currency = ""
	}
	return p
}

// AdjustedLimit is one limit scaled by pct percent, floored at one minor unit.
func AdjustedLimit(limit int64, pct float64) int64 {
	next := limit + int64(math.Round(float64(limit)*pct/100))
	if next < 1 {
		return 1
	}
	return next
}
