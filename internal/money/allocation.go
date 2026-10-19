// SPDX-License-Identifier: MIT

package money

import (
	"errors"
	"fmt"
	"math"
	"math/bits"
)

// ErrInvalidWeights is returned for empty, negative or all-zero weights.
var ErrInvalidWeights = errors.New("money: weights must be nonnegative with a positive sum")

// Allocate apportions total in proportion to nonnegative weights using largest
// remainders (Hamilton's method). Shares have total's sign and sum exactly to
// total, including MinInt64. Equal remainders favor the lowest input index.
// Zero weights receive zero. Empty/all-zero weights and weight-sum overflow are
// errors, even when total is zero. Inputs are never mutated.
//
// Products use unsigned 128-bit intermediates. Stable radix ordering and fewer
// than len(weights) remainder awards keep time and space O(len(weights)),
// independent of total. This apportionment policy does not change the explicit
// half-away-from-zero rounding policy used by monetary conversions.
func Allocate(total int64, weights []int64) ([]int64, error) {
	var weightSum int64
	for i, weight := range weights {
		if weight < 0 {
			return nil, fmt.Errorf("%w: index %d", ErrInvalidWeights, i)
		}
		var err error
		weightSum, err = Add(weightSum, weight)
		if err != nil {
			return nil, fmt.Errorf("money: weight sum at index %d: %w", i, err)
		}
	}
	if weightSum == 0 {
		return nil, ErrInvalidWeights
	}
	shares := make([]int64, len(weights))
	if total == 0 {
		return shares, nil
	}
	amount, denominator := magnitude(total), uint64(weightSum)
	quotients := make([]uint64, len(weights))
	remainders := make([]uint64, len(weights))
	left := amount
	for i, weight := range weights {
		hi, lo := bits.Mul64(amount, uint64(weight)) // #nosec G115 -- weight is nonnegative after validation at line 23 and fits uint64.
		// weight <= weightSum, so the quotient is <= amount <= 2^63.
		// Consequently hi < denominator, as required by bits.Div64.
		q, r := bits.Div64(hi, lo, denominator)
		quotients[i], remainders[i] = q, r
		left -= q // Floor shares cannot sum to more than amount.
	}
	if left > 0 {
		order := remainderOrder(remainders)
		// The sum of n fractional remainders is strictly less than n.
		for i := uint64(0); i < left; i++ {
			quotients[order[i]]++
		}
	}
	for i, q := range quotients {
		if total < 0 {
			if q == uint64(1)<<63 {
				shares[i] = math.MinInt64
			} else {
				shares[i] = -int64(q) // #nosec G115 -- q is below 2^63 because q == 2^63 was handled above.
			}
		} else {
			shares[i] = int64(q) // #nosec G115 -- q is at most MaxInt64 because total is positive int64.
		}
	}
	return shares, nil
}

// remainderOrder performs eight stable byte passes, largest remainder first.
// Stability preserves the original (lowest-index-first) ordering for ties.
func remainderOrder(remainders []uint64) []int {
	order := make([]int, len(remainders))
	scratch := make([]int, len(remainders))
	for i := range order {
		order[i] = i
	}
	for shift := uint(0); shift < 64; shift += 8 {
		var offsets [256]int
		for _, index := range order {
			key := 255 - byte(remainders[index]>>shift) // #nosec G115 -- this intentional narrowing extracts only the current 8-bit radix digit.
			offsets[key]++
		}
		position := 0
		for key, count := range offsets {
			offsets[key] = position
			position += count
		}
		for _, index := range order {
			key := 255 - byte(remainders[index]>>shift) // #nosec G115 -- this intentional narrowing extracts only the current 8-bit radix digit.
			scratch[offsets[key]] = index
			offsets[key]++
		}
		order, scratch = scratch, order
	}
	return order
}
