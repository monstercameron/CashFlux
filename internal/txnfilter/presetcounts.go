// SPDX-License-Identifier: MIT

package txnfilter

import (
	"strings"
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
	// LargeMinor is the Large preset threshold in Labels.BaseCurrency. When
	// positive, it takes precedence over LargeFor.
	LargeMinor int64
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
	return CountPresetsWithLabels(txns, c, s, Labels{})
}

// CountPresetsWithLabels reports preset counts using the same alias and currency
// context as ApplyWithLabels. It scans the input once and does not sort discarded
// result sets.
func CountPresetsWithLabels(txns []domain.Transaction, c Criteria, s PresetScope, labels Labels) PresetCounts {
	var out PresetCounts
	uncategorized := compileCriteria(c.Without(FieldUncategorized), labels)
	review := compileCriteria(c.Without(FieldTag), labels)
	large := compileCriteria(c.Without(FieldAmountMin), labels)
	month := compileCriteria(c.Without(FieldFrom).Without(FieldTo), labels)
	for _, t := range txns {
		if uncategorized.matches(t) && !t.IsTransfer() && hasUncategorizedCategory(t) {
			out.Uncategorized++
		}
		if s.ReviewTag != "" && review.matches(t) {
			for _, tag := range t.Tags {
				if tagEqual(tag, s.ReviewTag) {
					out.NeedsReview++
					break
				}
			}
		}
		if (s.LargeMinor > 0 || s.LargeFor != nil) && large.matches(t) {
			amount, err := amountInBase(t, labels)
			if err != nil {
				if labels.OnConversionError != nil {
					labels.OnConversionError(t, err)
				}
			} else if threshold := presetLargeThreshold(t, s); amount >= threshold {
				out.Large++
			}
		}
		if !s.MonthStart.IsZero() && !s.MonthEnd.IsZero() && month.matches(t) {
			if !t.Date.Before(s.MonthStart) && t.Date.Before(s.MonthEnd) {
				out.ThisMonth++
			}
		}
	}
	return out
}

func presetLargeThreshold(t domain.Transaction, s PresetScope) int64 {
	if s.LargeMinor > 0 {
		return s.LargeMinor
	}
	return s.LargeFor(t)
}

func tagEqual(a, b string) bool { return strings.EqualFold(a, b) }
