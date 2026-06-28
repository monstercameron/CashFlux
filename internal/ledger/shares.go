// SPDX-License-Identifier: MIT

package ledger

import (
	"sort"

	"github.com/monstercameron/CashFlux/internal/split"
)

// SplitByShares apportions amountMinor across the given member IDs by their
// integer percentage-point shares using the largest-remainder (Hamilton) method,
// guaranteeing the parts sum to exactly amountMinor.
//
// shares maps member ID → percentage points (values must sum to 100 when the
// map is non-empty; callers are responsible for enforcing the invariant).
// An empty or nil shares map returns an empty result.
//
// Negative amounts are handled by apportioning the magnitude and negating: the
// member with the largest remainder still receives the extra −1. Ties in
// remainder are broken by member ID in ascending lexical order (deterministic).
//
// Delegates to split.ByWeights for the Hamilton apportionment; this function
// is a thin adapter that converts the map representation and handles the sign.
func SplitByShares(amountMinor int64, shares map[string]int) map[string]int64 {
	if len(shares) == 0 {
		return map[string]int64{}
	}

	// Collect and sort member IDs for deterministic tie-breaking.
	// ByWeights breaks ties by slice index, so pre-sorting by ID gives
	// identical tie-break semantics to the old direct implementation.
	memberIDs := make([]string, 0, len(shares))
	for id := range shares {
		memberIDs = append(memberIDs, id)
	}
	sort.Strings(memberIDs)

	// Work on the absolute value; restore the sign at the end.
	// split.ByWeights clamps negative totals to zero, so we pass the
	// magnitude and flip the sign ourselves.
	sign := int64(1)
	abs := amountMinor
	if abs < 0 {
		sign = -1
		abs = -abs
	}

	weighted := make([]split.WeightedMember, len(memberIDs))
	for i, id := range memberIDs {
		weighted[i] = split.WeightedMember{MemberID: id, Weight: int64(shares[id])}
	}

	result := split.ByWeights(abs, weighted)
	out := make(map[string]int64, len(memberIDs))
	for _, s := range result {
		out[s.MemberID] = s.Amount * sign
	}
	return out
}
