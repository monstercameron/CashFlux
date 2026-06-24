// SPDX-License-Identifier: MIT

package subscriptions

import "time"

// AnnualSavings returns the total annual cost of the subscriptions whose names
// appear in selected (keyed by Subscription.Name, exact match). Only entries
// where selected[name] is true contribute to the sum. The result is in the same
// base-currency minor units as AnnualAmount.
func AnnualSavings(subs []Subscription, selected map[string]bool) int64 {
	var sum int64
	for _, s := range subs {
		if selected[s.Name] {
			sum += s.AnnualAmount()
		}
	}
	return sum
}

// NeedsReview returns true when a subscription appears overdue for a check-in —
// that is, when its most recent charge (Last) is older than two full cadence
// intervals before now. This is a low-pressure signal: a subscription that is
// still charging on schedule never triggers it; one whose Last date is stale
// (e.g. a trial that quietly stopped, or a charge the user forgot about) does.
//
// Two-interval thresholds by cadence:
//   - Weekly:  Last is more than 14 days before now
//   - Monthly: Last is more than 62 days before now  (≈2 × 31-day month)
//   - Yearly:  Last is more than 730 days before now (≈2 × 365 days)
func NeedsReview(s Subscription, now time.Time) bool {
	var threshold time.Duration
	switch s.Cadence {
	case CadenceWeekly:
		threshold = 14 * 24 * time.Hour
	case CadenceYearly:
		threshold = 730 * 24 * time.Hour
	default: // monthly
		threshold = 62 * 24 * time.Hour
	}
	return now.Sub(s.Last) > threshold
}
