// SPDX-License-Identifier: MIT

package orderimport

import (
	"sort"
	"time"
)

// DateWindowDays is how far a charge may sit from the order date and still be a
// candidate (±3 days; a card posts a day or two after the order).
const DateWindowDays = 3

// driftToleranceMinor is the largest order-total-vs-charge gap still treated as a
// single match (with the drift stated). A larger gap means gift-card / promo
// credit covered part of the order and no card charge matches the full total.
const driftToleranceMinor = 5

// maxSubsetCharges bounds the multi-shipment subset-sum search: an order that
// posted as several card charges (one per shipment) rarely exceeds three.
const maxSubsetCharges = 3

// Charge is one candidate card transaction the matcher may assign to an order.
// AmountMinor is signed as it reads in the ledger (an expense is negative); the
// matcher compares magnitudes.
type Charge struct {
	TxnID       string
	Date        time.Time
	AmountMinor int64
	Currency    string
}

// MatchKind classifies how an order matched (or didn't).
type MatchKind int

const (
	// MatchNone means no card charge (or combination) covers the order.
	MatchNone MatchKind = iota
	// MatchSingle means exactly one charge paid the order (offer the item split).
	MatchSingle
	// MatchMulti means several shipment charges together paid the order (offer an
	// XC1 order group).
	MatchMulti
)

// Match is the matcher's decision for one order: which charges settle it, the
// matched magnitude, and the drift between the order total and what the cards
// actually charged (order total − matched charges). A positive drift means the
// cards charged LESS than the order total — a gift card or promo covered the
// rest; it is surfaced plainly, never hidden.
type Match struct {
	Order        Order
	Kind         MatchKind
	TxnIDs       []string
	MatchedMinor int64
	DriftMinor   int64
}

// magnitude returns the absolute value of a signed minor amount.
func magnitude(v int64) int64 {
	if v < 0 {
		return -v
	}
	return v
}

// withinWindow reports whether a charge date is within DateWindowDays of the
// order date (calendar days, zone-insensitive). An order with no parsed date
// accepts any charge date (best-effort).
func withinWindow(orderDate, chargeDate time.Time) bool {
	if orderDate.IsZero() {
		return true
	}
	od := time.Date(orderDate.Year(), orderDate.Month(), orderDate.Day(), 0, 0, 0, 0, time.UTC)
	cd := time.Date(chargeDate.Year(), chargeDate.Month(), chargeDate.Day(), 0, 0, 0, 0, time.UTC)
	diff := int(cd.Sub(od).Hours() / 24)
	if diff < 0 {
		diff = -diff
	}
	return diff <= DateWindowDays
}

// MatchOrder assigns card charges to one order: an exact single-charge total
// first (±driftTolerance), then a subset of up to maxSubsetCharges charges whose
// magnitudes sum to the order total (multi-shipment), then a single closest
// charge within the drift tolerance (gift-card drift). used marks charges
// already assigned to another order so a charge is never double-counted. The
// returned Match has Kind MatchNone when nothing covers the order.
func MatchOrder(o Order, charges []Charge, used map[string]bool) Match {
	total := o.TotalMinor
	// Restrict to same-currency, in-window, unused charges.
	var cand []Charge
	for _, c := range charges {
		if used[c.TxnID] {
			continue
		}
		if o.Currency != "" && c.Currency != "" && o.Currency != c.Currency {
			continue
		}
		if !withinWindow(o.Date, c.Date) {
			continue
		}
		cand = append(cand, c)
	}
	// Deterministic order: closest to the order date, then largest magnitude, then id.
	sort.SliceStable(cand, func(i, j int) bool {
		if magnitude(cand[i].AmountMinor) != magnitude(cand[j].AmountMinor) {
			return magnitude(cand[i].AmountMinor) > magnitude(cand[j].AmountMinor)
		}
		return cand[i].TxnID < cand[j].TxnID
	})

	// 1) Exact single charge for the whole total.
	for _, c := range cand {
		if diff := magnitude(c.AmountMinor) - total; diff <= driftToleranceMinor && diff >= -driftToleranceMinor {
			return Match{Order: o, Kind: MatchSingle, TxnIDs: []string{c.TxnID},
				MatchedMinor: magnitude(c.AmountMinor), DriftMinor: total - magnitude(c.AmountMinor)}
		}
	}

	// 2) Subset of 2..maxSubsetCharges charges summing to the total (multi-shipment).
	if subset, sum, ok := findSubset(cand, total, maxSubsetCharges); ok {
		ids := make([]string, len(subset))
		for i, c := range subset {
			ids[i] = c.TxnID
		}
		return Match{Order: o, Kind: MatchMulti, TxnIDs: ids,
			MatchedMinor: sum, DriftMinor: total - sum}
	}

	// 3) Single charge within drift tolerance-of-total is handled in (1); anything
	// larger is a real gift-card/promo drift. Offer the closest single charge that
	// is not larger than the total by more than a small margin, stating the drift.
	best := -1
	var bestDiff int64
	for i, c := range cand {
		d := total - magnitude(c.AmountMinor) // positive when charge < total (gift card)
		if d <= 0 {
			continue // charge >= total but not within exact tolerance — not this order
		}
		if best < 0 || d < bestDiff {
			best, bestDiff = i, d
		}
	}
	if best >= 0 {
		c := cand[best]
		return Match{Order: o, Kind: MatchSingle, TxnIDs: []string{c.TxnID},
			MatchedMinor: magnitude(c.AmountMinor), DriftMinor: bestDiff}
	}

	return Match{Order: o, Kind: MatchNone, DriftMinor: total}
}

// findSubset searches for a subset of size 2..maxN of charges whose magnitudes
// sum exactly to target (±driftTolerance). It returns the first such subset in
// deterministic order, the summed magnitude, and ok. Bounded at maxN (≤3) so the
// search is cheap.
func findSubset(cand []Charge, target int64, maxN int) ([]Charge, int64, bool) {
	n := len(cand)
	mags := make([]int64, n)
	for i, c := range cand {
		mags[i] = magnitude(c.AmountMinor)
	}
	near := func(sum int64) bool {
		d := sum - target
		return d <= driftToleranceMinor && d >= -driftToleranceMinor
	}
	// size 2
	for i := 0; i < n; i++ {
		for j := i + 1; j < n; j++ {
			if s := mags[i] + mags[j]; near(s) {
				return []Charge{cand[i], cand[j]}, s, true
			}
		}
	}
	if maxN < 3 {
		return nil, 0, false
	}
	// size 3
	for i := 0; i < n; i++ {
		for j := i + 1; j < n; j++ {
			for k := j + 1; k < n; k++ {
				if s := mags[i] + mags[j] + mags[k]; near(s) {
					return []Charge{cand[i], cand[j], cand[k]}, s, true
				}
			}
		}
	}
	return nil, 0, false
}

// MatchOrders matches every order against the charges, greatest-total order
// first (large multi-shipment orders claim their charges before small ones can
// grab a shared charge), and never assigns a charge to two orders. Orders are
// returned in their input order, each with its Match (Kind MatchNone when
// unmatched).
func MatchOrders(orders []Order, charges []Charge) []Match {
	idx := make([]int, len(orders))
	for i := range orders {
		idx[i] = i
	}
	sort.SliceStable(idx, func(a, b int) bool {
		return orders[idx[a]].TotalMinor > orders[idx[b]].TotalMinor
	})
	used := map[string]bool{}
	results := make([]Match, len(orders))
	for _, i := range idx {
		m := MatchOrder(orders[i], charges, used)
		for _, id := range m.TxnIDs {
			used[id] = true
		}
		results[i] = m
	}
	return results
}
