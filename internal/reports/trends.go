// SPDX-License-Identifier: MIT

package reports

import (
	"sort"
	"time"

	"github.com/monstercameron/CashFlux/internal/currency"
	"github.com/monstercameron/CashFlux/internal/domain"
	"github.com/monstercameron/CashFlux/internal/ledger"
)

// CategoryTrend is one category's spend across consecutive reporting buckets — the
// data behind a sparkline plus its headline change. Spend holds the absolute
// expense for each bucket in base-currency minor units, oldest first (length ==
// number of buckets); Total is their sum; DeltaPct is the percent change from the
// first bucket to the last, meaningful only when HasDelta is true. CategoryID is
// empty for uncategorized spend; the caller resolves names.
type CategoryTrend struct {
	CategoryID string
	Spend      []int64
	Total      int64
	DeltaPct   int64
	HasDelta   bool
}

// CategoryTrends builds a per-category spend sparkline across the consecutive
// buckets defined by bounds — bucket i is [bounds[i], bounds[i+1]) — oldest first,
// for the "category trends" report. Each category gets one Spend value per bucket
// (absolute expense in the base currency; income and transfers excluded), its
// Total over the window, and the percent change from the first bucket to the last
// so the biggest movers stand out. Categories are sorted by Total descending,
// ties broken by category id for determinism. Fewer than two bounds — and so no
// complete bucket — yields no trends.
func CategoryTrends(txns []domain.Transaction, bounds []time.Time, rates currency.Rates) ([]CategoryTrend, error) {
	n := len(bounds) - 1
	if n < 1 {
		return nil, nil
	}

	buckets := make([]map[string]int64, n)
	ids := map[string]struct{}{}
	for i := 0; i < n; i++ {
		b, err := categoryTotals(txns, bounds[i], bounds[i+1], rates)
		if err != nil {
			return nil, err
		}
		buckets[i] = b
		for id := range b {
			ids[id] = struct{}{}
		}
	}

	out := make([]CategoryTrend, 0, len(ids))
	for id := range ids {
		trend := CategoryTrend{CategoryID: id, Spend: make([]int64, n)}
		for i, b := range buckets {
			amt := b[id]
			trend.Spend[i] = amt
			trend.Total += amt
		}
		trend.DeltaPct, trend.HasDelta = ledger.PercentChange(trend.Spend[n-1], trend.Spend[0])
		out = append(out, trend)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Total != out[j].Total {
			return out[i].Total > out[j].Total
		}
		return out[i].CategoryID < out[j].CategoryID
	})
	return out, nil
}
