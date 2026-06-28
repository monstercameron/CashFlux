// SPDX-License-Identifier: MIT

package ledger

import "sort"

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
func SplitByShares(amountMinor int64, shares map[string]int) map[string]int64 {
	if len(shares) == 0 {
		return map[string]int64{}
	}

	// Collect and sort member IDs for deterministic tie-breaking.
	members := make([]string, 0, len(shares))
	for id := range shares {
		members = append(members, id)
	}
	sort.Strings(members)

	out := make(map[string]int64, len(members))

	// Work on the absolute value; restore the sign at the end.
	sign := int64(1)
	abs := amountMinor
	if abs < 0 {
		sign = -1
		abs = -abs
	}

	// Floor allocation: each member gets floor(abs * share / 100).
	// Integer division truncates toward zero, which equals floor for abs ≥ 0.
	var allocated int64
	for _, id := range members {
		q := abs * int64(shares[id]) / 100
		out[id] = q
		allocated += q
	}

	// Distribute the leftover units (abs - allocated) to the members with the
	// largest remainders. Remainder for member i = (abs * shares[i]) % 100.
	remaining := abs - allocated
	if remaining > 0 {
		type mr struct {
			id  string
			rem int64
		}
		mrs := make([]mr, len(members))
		for i, id := range members {
			mrs[i] = mr{id: id, rem: (abs * int64(shares[id])) % 100}
		}
		// Stable sort preserves the lexical tie-break order established by the
		// earlier sort.Strings call.
		sort.SliceStable(mrs, func(i, j int) bool { return mrs[i].rem > mrs[j].rem })
		for i := int64(0); i < remaining; i++ {
			out[mrs[i].id]++
		}
	}

	// Restore sign.
	if sign < 0 {
		for id := range out {
			out[id] = -out[id]
		}
	}

	return out
}
