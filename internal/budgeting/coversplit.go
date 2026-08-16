// SPDX-License-Identifier: MIT

package budgeting

import "sort"

// CoverShare is one source's contribution to a funded cover or top-up.
type CoverShare struct {
	ID     string
	Amount int64 // minor units
}

// CoverSplitInput is one selectable source: what it can give, and how heavily
// the user weighted it.
type CoverSplitInput struct {
	ID string
	// Cap is the most this source can give without being left overspent or with
	// a non-positive limit — budgeting.SourceFunds.Movable.
	Cap int64
	// Weight is the user's ratio for this source; 0 or less reads as 1.
	Weight int
	// Pinned marks a "use all it has" source: it gives its whole Cap before the
	// remainder is split across the others.
	Pinned bool
}

// CoverSplit divides total across the given sources, never asking any of them
// for more than it can give (C606).
//
// The weighted split it replaces had no ceiling: a source could be assigned more
// than it had, and the write then either failed validation (permanent move) or —
// on the this-period path, which had no guard at all — silently pushed the
// source's effective cap below what it had already spent, leaving it overspent
// with nothing on screen having said so.
//
// Capping alone is not enough, because a capped source leaves the total short.
// The remainder is redistributed across the sources that still have room, in
// weight order, repeatedly until either the total is met or every source is at
// its cap. short reports what could not be funded, so the caller can say so
// rather than quietly moving less than the user asked for.
func CoverSplit(total int64, srcs []CoverSplitInput) (shares map[string]int64, short int64) {
	if total <= 0 || len(srcs) == 0 {
		return nil, total
	}
	out := make(map[string]int64, len(srcs))
	capOf := make(map[string]int64, len(srcs))
	remaining := total

	// Pinned sources give everything they have first — that is what pinning means.
	var free []CoverSplitInput
	for _, s := range srcs {
		c := s.Cap
		if c < 0 {
			c = 0
		}
		capOf[s.ID] = c
		if s.Pinned {
			give := min64(c, remaining)
			out[s.ID] = give
			remaining -= give
			continue
		}
		free = append(free, s)
	}
	if remaining <= 0 {
		return out, 0
	}

	// Weighted rounds over the sources with room left. Each round distributes the
	// outstanding amount by weight and clips at each cap; whatever a clipped
	// source could not take rolls into the next round. Sorting by (weight desc,
	// id) keeps the result deterministic — a split that changes between renders
	// would make the preview and the write disagree.
	sort.SliceStable(free, func(i, j int) bool {
		if free[i].Weight != free[j].Weight {
			return free[i].Weight > free[j].Weight
		}
		return free[i].ID < free[j].ID
	})
	for round := 0; round < len(free)+1 && remaining > 0; round++ {
		var open []CoverSplitInput
		totalWeight := 0
		for _, s := range free {
			if out[s.ID] < capOf[s.ID] {
				open = append(open, s)
				totalWeight += weightOf(s)
			}
		}
		if len(open) == 0 || totalWeight == 0 {
			break
		}
		before := remaining
		var assigned int64
		for i, s := range open {
			want := remaining * int64(weightOf(s)) / int64(totalWeight)
			if i == len(open)-1 {
				want = remaining - assigned
			}
			assigned += want
			room := capOf[s.ID] - out[s.ID]
			give := min64(want, room)
			out[s.ID] += give
		}
		// Recompute what is still outstanding from what was actually given.
		var given int64
		for _, v := range out {
			given += v
		}
		remaining = total - given
		if remaining == before {
			break // no progress — every open source is at its cap
		}
	}
	return out, max64(remaining, 0)
}

func weightOf(s CoverSplitInput) int {
	if s.Weight <= 0 {
		return 1
	}
	return s.Weight
}

func min64(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}

func max64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}
