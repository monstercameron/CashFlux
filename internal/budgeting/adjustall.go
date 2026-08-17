// SPDX-License-Identifier: MIT

package budgeting

import (
	"math"
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/money"
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

// SkipReason says why a budget is left out of a bulk adjustment.
type SkipReason string

// The reasons a budget is left out.
const (
	// SkipNothingToScale: the figure the adjustment would scale is already at or
	// below zero, so there is nothing to take a percentage of.
	SkipNothingToScale SkipReason = "nothing-to-scale"
	// SkipUnknownOverlay: this budget's period state could not be resolved — how
	// much rollover carry-in and one-off change sits on top of its base limit —
	// so neither what a this-period change would scale nor what a permanent one
	// would leave behind can be stated honestly.
	SkipUnknownOverlay SkipReason = "unknown-overlay"
	// SkipWouldInvert: the write would leave this budget's effective cap at or
	// below zero — because a one-off change already recorded for this period
	// stacks on top of the new limit. A bulk lower may shrink a budget; it may
	// not turn its plan negative.
	SkipWouldInvert SkipReason = "would-invert"
)

// AdjustOverlays maps budget ID to that budget's overlay for a period: how far
// its effective cap sits above (or below) its stored base limit, being rollover
// carry-in plus any one-off change already recorded.
//
// It is a named type so that handing this API a map of effective CAPS — the
// other obvious map[string]int64 about the same budgets, differing by exactly the
// base limit — cannot compile. That substitution has already happened once, at a
// call site that only displayed a count; the next one might not be.
type AdjustOverlays map[string]int64

// AdjustSkip is one budget the adjustment leaves alone, and why.
//
// Skipping used to be silent: a budget with nothing to scale simply did not
// appear in the preview, and the count quietly disagreed with "every budget". A
// bulk edit that omits something has to say which and why, or the omission reads
// as a bug the next time the user counts the rows.
type AdjustSkip struct {
	Budget domain.Budget
	Reason SkipReason
}

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
	// Skipped lists the budgets the adjustment will not touch, and why, so the
	// form can say so instead of quietly showing fewer rows than expected.
	Skipped []AdjustSkip
}

// SkippedFor returns the budgets left out for one reason.
func (p AdjustPreview) SkippedFor(reason SkipReason) []AdjustSkip {
	var out []AdjustSkip
	for _, s := range p.Skipped {
		if s.Reason == reason {
			out = append(out, s)
		}
	}
	return out
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

// AdjustScope is how long a bulk adjustment lasts (C671).
//
// The form used to have one behaviour and never named it: every apply rewrote
// base limits, so a user reconciling a single underfunded month rewrote their
// plan for every month after it. Reach and magnitude are different questions —
// a 5% cut that never ends can matter more than a 40% cut that reverts in a
// fortnight — and the form has to ask both.
type AdjustScope string

// The scopes a bulk adjustment can have.
const (
	// AdjustThisPeriod records the change as a one-time period boost per budget —
	// the mechanism top-up and release already use — so it applies to the period
	// being viewed and reverts at the next boundary, leaving the plan intact.
	AdjustThisPeriod AdjustScope = "this-period"
	// AdjustEveryPeriod rewrites each budget's base limit, so this period and
	// every period after it start from the new number.
	AdjustEveryPeriod AdjustScope = "every-period"
)

// Valid reports whether s is a scope the form may apply.
func (s AdjustScope) Valid() bool { return s == AdjustThisPeriod || s == AdjustEveryPeriod }

// IsPermanent reports whether the scope outlives the period being viewed.
func (s AdjustScope) IsPermanent() bool { return s == AdjustEveryPeriod }

// AdjustNeedsAck reports whether an adjustment must be acknowledged before it is
// applied, rather than merely previewed.
//
// A change to every future period always needs one, whatever its size (C671):
// what makes that write consequential is its reach, not its magnitude, and
// magnitude was the only thing the old rule could see. A this-period change is
// still gated on size, because it reverts on its own.
func AdjustNeedsAck(pct float64, scope AdjustScope) bool {
	return scope.IsPermanent() || IsLargeAdjust(pct)
}

// ApplyAdjust returns each affected budget as the adjustment would store it,
// leaving the caller only the persistence.
//
// The two scopes are two different writes, not one write with a flag: a permanent
// adjustment rewrites the base limit, while a this-period one records a one-time
// boost of the delta, so it lapses at the next period boundary. periodStart
// resolves a budget's current period start (cadences differ, and the household's
// week-start and pay-cycle anchor live in the view layer); it is required for
// AdjustThisPeriod and unused otherwise.
//
// p MUST have been built by AdjustAllPreviewFor under the same scope — the
// delta a this-period write records is only correct if the preview measured it
// against the effective cap the boost lands on.
func ApplyAdjust(p AdjustPreview, scope AdjustScope, periodStart func(domain.Budget) time.Time) []domain.Budget {
	out := make([]domain.Budget, 0, len(p.Lines))
	for _, line := range p.Lines {
		b := line.Budget
		if scope == AdjustThisPeriod {
			if periodStart == nil {
				continue // no period to attach the boost to; writing the base limit here would be the very thing the scope excludes
			}
			out = append(out, b.WithPeriodBoost(periodStart(b), line.Delta()))
			continue
		}
		b.Limit = money.New(line.After, b.Limit.Currency)
		out = append(out, b)
	}
	return out
}

// AdjustAllPreviewFor computes the before/after for an adjustment under a given
// scope, on the figure that scope will actually write.
//
// The two scopes scale two different numbers, and conflating them is how the
// preview came to promise something the write could not deliver. A permanent
// adjustment moves the stored base limit, so that is its basis. A this-period one
// records a boost, and a boost lands on the EFFECTIVE cap — base plus rollover
// carry-in plus whatever boost is already recorded — which is the figure the
// card scores against. Scaling the base and calling the difference a boost looked
// right on a budget carrying nothing and was wrong on every budget carrying
// something: a $400 budget with $50 carried in, lowered 20%, previewed
// "$400 → $320" and produced a cap of $370. Applying twice in one period was
// worse, because the base never moved: the second preview looked identical to the
// first while the boost accumulated, and enough repeats drove the cap negative.
//
// overlays maps budget ID to that budget's OVERLAY for the period — the amount
// its effective cap sits above (or below) its stored base limit, being rollover
// carry-in plus any one-off change already recorded. It is REQUIRED under both
// scopes: a this-period adjustment scales base+overlay, and a permanent one has
// to check that the base it writes still clears the overlay riding on top of it.
//
// It is an OVERLAY rather than a cap deliberately. A cap can only be known where
// the budget's whole status could be evaluated, which couples eligibility to
// spend and FX data a permanent base-limit rewrite has no business depending on —
// a budget carrying nothing at all would be dropped from the tool because an
// unrelated transaction lacked an exchange rate. An overlay of zero is knowable
// without any of that.
//
// A budget missing from overlays is skipped with SkipUnknownOverlay under either
// scope, never previewed with the check quietly disabled. Making the guard
// conditional on "did we happen to know the overlay" is the same defect as not
// having it: the budget nobody could evaluate is exactly the one whose overlay
// nobody has looked at.
//
// Budgets whose basis is non-positive are skipped too — there is nothing left to
// scale — and a reduction floors at one minor unit, so a bulk lower may shrink a
// budget but never drive its plan to zero or below.
func AdjustAllPreviewFor(budgets []domain.Budget, pct float64, scope AdjustScope, overlays AdjustOverlays) AdjustPreview {
	var p AdjustPreview
	for _, b := range budgets {
		overlay, known := overlays[b.ID]
		if !known {
			p.Skipped = append(p.Skipped, AdjustSkip{Budget: b, Reason: SkipUnknownOverlay})
			continue
		}
		before := b.Limit.Amount
		if scope == AdjustThisPeriod {
			before = b.Limit.Amount + overlay
		}
		if before <= 0 {
			p.Skipped = append(p.Skipped, AdjustSkip{Budget: b, Reason: SkipNothingToScale})
			continue
		}
		after := AdjustedLimit(before, pct)
		// The cap the write would leave behind. Under this-period scope `after` IS
		// the cap and is already floored; under a permanent one the overlay rides
		// along on top of the new limit.
		if scope == AdjustEveryPeriod && after+overlay < 1 {
			p.Skipped = append(p.Skipped, AdjustSkip{Budget: b, Reason: SkipWouldInvert})
			continue
		}
		p.Lines = append(p.Lines, AdjustLine{Budget: b, Before: before, After: after})
		p.TotalBefore += before
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
