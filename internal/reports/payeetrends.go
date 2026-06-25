// SPDX-License-Identifier: MIT

package reports

import (
	"sort"
	"strings"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/dateutil"
	"github.com/monstercameron/CashFlux/internal/domain"
)

// PayeeTrend is one payee's expense spend across consecutive reporting buckets —
// the per-merchant companion to CategoryTrend. Payee is the display name (first
// spelling seen, case-insensitive dedup, falling back to Desc when both Payee and
// Desc are blank). Spend holds the absolute expense in base-currency minor units
// for each bucket, oldest first (length == number of buckets); the slice aligns
// exactly with the bounds passed to PayeeTrends, mirroring CategoryTrends.
type PayeeTrend struct {
	Payee string
	Spend []int64
}

// payeeTotals sums absolute expense amounts by payee key over [start, end),
// converted to the base currency. Transfers and income are excluded.
// The key is strings.ToLower(strings.TrimSpace(desc)); the display name is the
// first spelling encountered for that key. This mirrors TopPayees exactly.
func payeeTotals(txns []domain.Transaction, start, end time.Time, rates currency.Rates) (map[string]int64, map[string]string, error) {
	totals := map[string]int64{}
	names := map[string]string{} // key → display name (first seen)
	for _, t := range txns {
		if !t.IsExpense() || !dateutil.InRange(t.Date, start, end) {
			continue
		}
		conv, err := rates.Convert(t.Amount, rates.Base)
		if err != nil {
			return nil, nil, err
		}
		name := strings.TrimSpace(t.Desc)
		key := strings.ToLower(name)
		if _, ok := names[key]; !ok {
			names[key] = name
		}
		totals[key] += conv.Abs().Amount
	}
	return totals, names, nil
}

// PayeeTrends builds a per-payee expense sparkline across the consecutive buckets
// defined by bounds — bucket i is [bounds[i], bounds[i+1]) — oldest first. It
// selects the top-N payees by their total spend over the entire window (using the
// same payee normalization as TopPayees: TrimSpace + case-insensitive grouping,
// Desc field, first spelling kept for display), then for each selected payee
// returns a Spend series aligned to the same buckets so the result pairs directly
// with CategoryTrends output. Only expenses are counted; income and transfers are
// excluded; amounts are converted to the base currency. topN <= 0 returns all
// payees. Fewer than two bounds — and so no complete bucket — yields nil.
func PayeeTrends(txns []domain.Transaction, bounds []time.Time, rates currency.Rates, topN int) ([]PayeeTrend, error) {
	n := len(bounds) - 1
	if n < 1 {
		return nil, nil
	}

	// Collect per-bucket totals and build the union of display names across all
	// buckets (earliest spelling wins per key, globally).
	buckets := make([]map[string]int64, n)
	allNames := map[string]string{} // key → display name (first seen across all buckets)
	for i := 0; i < n; i++ {
		totals, names, err := payeeTotals(txns, bounds[i], bounds[i+1], rates)
		if err != nil {
			return nil, err
		}
		buckets[i] = totals
		for key, name := range names {
			if _, exists := allNames[key]; !exists {
				allNames[key] = name
			}
		}
	}

	// Sum each key's spend over the full window to rank candidates.
	totalByKey := map[string]int64{}
	for _, b := range buckets {
		for key, amt := range b {
			totalByKey[key] += amt
		}
	}

	// Collect all keys, sort by total descending (ties break by key for
	// determinism, mirroring TopPayees / CategoryTrends tie-breaking style).
	keys := make([]string, 0, len(totalByKey))
	for key := range totalByKey {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		if totalByKey[keys[i]] != totalByKey[keys[j]] {
			return totalByKey[keys[i]] > totalByKey[keys[j]]
		}
		return keys[i] < keys[j]
	})

	// Apply topN limit (same semantics as TopPayees: 0 means all).
	if topN > 0 && len(keys) > topN {
		keys = keys[:topN]
	}

	if len(keys) == 0 {
		return nil, nil
	}

	// Build result: one PayeeTrend per selected key.
	out := make([]PayeeTrend, 0, len(keys))
	for _, key := range keys {
		trend := PayeeTrend{
			Payee: allNames[key],
			Spend: make([]int64, n),
		}
		for i, b := range buckets {
			trend.Spend[i] = b[key]
		}
		out = append(out, trend)
	}
	return out, nil
}
