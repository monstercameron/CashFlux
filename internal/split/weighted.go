// SPDX-License-Identifier: MIT

package split

import "sort"

// WeightedMember is a member with a relative weight for a proportional split —
// for example a share count (2:1) or an income figure to split a cost in
// proportion to earnings. Non-positive weights receive nothing.
type WeightedMember struct {
	MemberID string
	Weight   int64
}

// ByWeights splits a non-negative total in proportion to each member's weight,
// distributing the rounding remainder by the largest-remainder (Hamilton) method
// — the leftover minor units go to the members whose exact shares had the biggest
// fractional parts — so the shares sum to total exactly. Members with a
// non-positive weight are kept in the result with a zero share. Order is
// preserved. Returns nil when there are no members or every weight is
// non-positive (no basis to divide on).
func ByWeights(total int64, members []WeightedMember) []Share {
	if total < 0 {
		total = 0
	}
	var sumW int64
	for _, m := range members {
		if m.Weight > 0 {
			sumW += m.Weight
		}
	}
	if sumW <= 0 {
		return nil
	}

	out := make([]Share, len(members))
	type frac struct {
		idx int
		rem int64
	}
	var fracs []frac
	var assigned int64
	for i, m := range members {
		out[i] = Share{MemberID: m.MemberID}
		if m.Weight <= 0 {
			continue
		}
		product := total * m.Weight
		out[i].Amount = product / sumW
		assigned += out[i].Amount
		fracs = append(fracs, frac{idx: i, rem: product % sumW})
	}

	// Hand the leftover (total − sum of floors, which is < number of weighted
	// members) one unit at a time to the largest remainders, ties by order.
	leftover := total - assigned
	sort.SliceStable(fracs, func(a, b int) bool {
		if fracs[a].rem != fracs[b].rem {
			return fracs[a].rem > fracs[b].rem
		}
		return fracs[a].idx < fracs[b].idx
	})
	for k := 0; k < int(leftover) && k < len(fracs); k++ {
		out[fracs[k].idx].Amount++
	}
	return out
}
