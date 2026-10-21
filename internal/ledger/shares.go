// SPDX-License-Identifier: MIT

package ledger

import (
	"fmt"
	"sort"

	"github.com/monstercameron/CashFlux/internal/money"
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
//
// Deprecated: use SplitBySharesChecked to retain invalid-share errors and
// correctly apportion amounts whose magnitude or weighted products exceed int64.
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

// SplitBySharesChecked apportions signed minor units by percentage-point shares
// with exact Hamilton remainders, including MinInt64 and large products. Equal
// remainders favor lexical member ID order. Empty shares return an empty map.
// Nonempty shares must be nonnegative and sum to 100; invalid inputs return an
// error wrapping money.ErrInvalidWeights. The caller's map is never modified.
func SplitBySharesChecked(amountMinor int64, shares map[string]int) (map[string]int64, error) {
	if len(shares) == 0 {
		return map[string]int64{}, nil
	}
	memberIDs := make([]string, 0, len(shares))
	for id := range shares {
		memberIDs = append(memberIDs, id)
	}
	sort.Strings(memberIDs)
	weights := make([]int64, len(memberIDs))
	var total int64
	for i, id := range memberIDs {
		weight := int64(shares[id])
		if weight < 0 || weight > 100 {
			return nil, fmt.Errorf("ledger: ownership share for member %s: %w", id, money.ErrInvalidWeights)
		}
		var err error
		total, err = money.Add(total, weight)
		if err != nil {
			return nil, fmt.Errorf("ledger: ownership share total: %w", err)
		}
		weights[i] = weight
	}
	if total != 100 {
		return nil, fmt.Errorf("ledger: ownership shares total %d, want 100: %w", total, money.ErrInvalidWeights)
	}
	parts, err := money.Allocate(amountMinor, weights)
	if err != nil {
		return nil, fmt.Errorf("ledger: ownership split: %w", err)
	}
	out := make(map[string]int64, len(memberIDs))
	for i, id := range memberIDs {
		out[id] = parts[i]
	}
	return out, nil
}
