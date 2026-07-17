// SPDX-License-Identifier: MIT

package split

import "fmt"

// PercentScale is the fixed-point scale for percentages: basis points, i.e.
// hundredths of a percent. "33.34%" is 3334 basis points; a full split is
// exactly 10000.
const PercentScale = int64(10000)

// ByPercents splits a non-negative total (integer minor units) across lines
// whose shares are given as basis points (hundredths of a percent). Every line
// must be positive and together they must total exactly 100.00% (10000 basis
// points) — a partial or overshooting split is an error, not a guess. The
// rounding remainder is distributed by the same largest-remainder method as
// ByWeights, so the returned amounts always sum to total exactly.
func ByPercents(total int64, basisPoints []int64) ([]int64, error) {
	if len(basisPoints) == 0 {
		return nil, fmt.Errorf("split by percents: no lines")
	}
	var sum int64
	for i, bp := range basisPoints {
		if bp <= 0 {
			return nil, fmt.Errorf("split by percents: line %d has a non-positive percentage", i+1)
		}
		sum += bp
	}
	if sum != PercentScale {
		return nil, fmt.Errorf("split by percents: percentages total %d basis points, want %d (100%%)", sum, PercentScale)
	}
	members := make([]WeightedMember, len(basisPoints))
	for i, bp := range basisPoints {
		members[i] = WeightedMember{MemberID: fmt.Sprintf("line-%d", i), Weight: bp}
	}
	shares := ByWeights(total, members)
	out := make([]int64, len(shares))
	for i, s := range shares {
		out[i] = s.Amount
	}
	return out, nil
}
