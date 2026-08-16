// SPDX-License-Identifier: MIT

package txnfilter

import (
	"time"

	"github.com/monstercameron/CashFlux/internal/domain"
)

// PresetCounts is the number of transactions each quick-filter preset would show
// if it were switched on right now, given everything else the user has narrowed to.
type PresetCounts struct {
	Uncategorized int
	NeedsReview   int
	Large         int
	ThisMonth     int
}

// PresetScope describes the preset dimensions a caller wants counted, and the
// values the presets write. It is passed in rather than hard-coded so the pure
// package stays free of the review-tag constant and the "large" threshold, both of
// which are UI policy.
type PresetScope struct {
	// ReviewTag is the tag the "Needs review" preset filters on.
	ReviewTag string
	// LargeMinor is the absolute-amount floor (minor units) the "Large" preset
	// applies, resolved per transaction currency by the caller's LargeFor.
	LargeFor func(t domain.Transaction) int64
	// MonthStart and MonthEnd bound the "This month" preset, half-open [start, end).
	MonthStart, MonthEnd time.Time
}

// CountPresets reports how many transactions each quick-filter preset matches
// WITHIN the caller's current scope.
//
// C568: the counts used to be taken over the whole ledger, so with a search active
// the chips beside it still read "Uncategorized 249 · Large 1665" — figures that
// described a population the user was not looking at, and that no click could
// produce. A count beside a filter is a promise about what pressing it yields, so
// each preset is now counted against the criteria MINUS its own dimension: turning
// it on then really does leave that many rows.
//
// Removing the preset's own dimension matters — counting inside a scope that
// already includes the preset would make an active chip report the rows it is
// currently showing rather than the rows it selects, and every inactive chip would
// read 0 the moment a different preset was engaged.
func CountPresets(txns []domain.Transaction, c Criteria, s PresetScope) PresetCounts {
	var out PresetCounts

	// Uncategorized: drop the preset itself, then count what has no category.
	base := c.Without(FieldUncategorized)
	for _, t := range Apply(txns, base) {
		if !t.IsTransfer() && t.CategoryID == "" {
			out.Uncategorized++
		}
	}

	// Needs review: the preset writes the tag dimension, so that is what is dropped.
	if s.ReviewTag != "" {
		for _, t := range Apply(txns, c.Without(FieldTag)) {
			for _, tag := range t.Tags {
				if tag == s.ReviewTag {
					out.NeedsReview++
					break
				}
			}
		}
	}

	// Large: the preset writes AmountMin, so an existing minimum is dropped first.
	if s.LargeFor != nil {
		for _, t := range Apply(txns, c.Without(FieldAmountMin)) {
			if AbsAmount(t) >= s.LargeFor(t) {
				out.Large++
			}
		}
	}

	// This month: the preset writes both date bounds, so both come off.
	if !s.MonthStart.IsZero() && !s.MonthEnd.IsZero() {
		dated := c.Without(FieldFrom).Without(FieldTo)
		for _, t := range Apply(txns, dated) {
			if !t.Date.Before(s.MonthStart) && t.Date.Before(s.MonthEnd) {
				out.ThisMonth++
			}
		}
	}

	return out
}
