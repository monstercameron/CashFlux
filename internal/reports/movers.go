// SPDX-License-Identifier: MIT

package reports

import "sort"

// absChange is the magnitude of a category's change versus the prior period.
func absChange(r CategorySpend) int64 {
	c := r.Amount - r.Prior
	if c < 0 {
		return -c
	}
	return c
}

// TopMovers returns the categories that changed the most versus the prior period
// — largest absolute change first — from a compared report (rows produced by
// SpendingByCategory with compare=true). Rows without a delta, or that didn't
// change, are excluded. n <= 0 returns every mover; otherwise at most n. Ties on
// the change magnitude are broken by category id so the order is deterministic.
func TopMovers(rows []CategorySpend, n int) []CategorySpend {
	movers := make([]CategorySpend, 0, len(rows))
	for _, r := range rows {
		if r.HasDelta && r.Amount != r.Prior {
			movers = append(movers, r)
		}
	}
	sort.Slice(movers, func(i, j int) bool {
		ai, aj := absChange(movers[i]), absChange(movers[j])
		if ai != aj {
			return ai > aj
		}
		return movers[i].CategoryID < movers[j].CategoryID
	})
	if n > 0 && len(movers) > n {
		movers = movers[:n]
	}
	return movers
}
