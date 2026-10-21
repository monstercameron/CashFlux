// SPDX-License-Identifier: MIT

package goals

import (
	"math/big"
	"math/bits"
	"sort"
)

// SplitMode selects how SplitEarmark distributes a target total across accounts.
type SplitMode string

const (
	// SplitEven spreads the total as equally as possible, waterfalling any overflow from
	// accounts that hit their cap onto the accounts that still have room.
	SplitEven SplitMode = "even"
	// SplitProportional splits the total in proportion to each account's available headroom
	// (bigger free balance → bigger share), so no account is asked for more than it has.
	SplitProportional SplitMode = "proportional"
)

// SplitEarmark distributes `total` (minor units) across accounts with the given per-account
// available headrooms (`avail`, minor units — already net of other goals' earmarks), and
// returns the per-account amounts to earmark. Guarantees:
//
//   - every out[i] is in [0, avail[i]] (never earmark more than an account can back);
//   - Σ out == min(total, Σ max(avail,0)) — it hands out the whole total, or all capacity
//     when the total exceeds what's available;
//   - index order is preserved and the result is deterministic (stable tiebreaks).
//
// Even mode spreads equally with a waterfall for capped accounts; proportional mode uses a
// largest-remainder apportionment by headroom. Negative avails are treated as 0.
func SplitEarmark(total int64, avail []int64, mode SplitMode) []int64 {
	n := len(avail)
	out := make([]int64, n)
	if n == 0 || total <= 0 {
		return out
	}
	cap := make([]int64, n)
	var capacity uint64
	capacityOverflow := false
	for i, a := range avail {
		if a > 0 {
			cap[i] = a
			var carry uint64
			capacity, carry = bits.Add64(capacity, uint64(a), 0)
			capacityOverflow = capacityOverflow || carry != 0
		}
	}
	if capacity == 0 && !capacityOverflow {
		return out
	}
	if !capacityOverflow && uint64(total) > capacity {
		total = int64(capacity)
	}
	if mode == SplitProportional {
		if capacityOverflow {
			return splitProportionalLarge(total, cap, out)
		}
		return splitProportional(total, cap, out, capacity)
	}
	return splitEven(total, cap, out)
}

// splitEven distributes total as equally as possible, giving each still-uncapped account an
// equal share each pass and waterfalling the remainder from capped accounts onto the rest.
func splitEven(total int64, cap []int64, out []int64) []int64 {
	remaining := total
	for remaining > 0 {
		active := 0
		for i := range cap {
			if out[i] < cap[i] {
				active++
			}
		}
		if active == 0 {
			break
		}
		share := remaining / int64(active)
		if share == 0 {
			// Fewer units left than active accounts — hand them out one at a time in order.
			for i := range cap {
				if remaining == 0 {
					break
				}
				if out[i] < cap[i] {
					out[i]++
					remaining--
				}
			}
			break
		}
		for i := range cap {
			if out[i] >= cap[i] {
				continue
			}
			give := share
			if room := cap[i] - out[i]; give > room {
				give = room
			}
			out[i] += give
			remaining -= give
		}
	}
	return out
}

// splitProportional apportions total by each account's share of capacity, using the
// largest-remainder method so the parts sum exactly to total (no rounding drift).
func splitProportional(total int64, cap []int64, out []int64, capacity uint64) []int64 {
	rem := make([]uint64, len(cap))
	var distributed int64
	for i, c := range cap {
		if c <= 0 {
			continue
		}
		hi, lo := bits.Mul64(uint64(total), uint64(c)) // #nosec G115 -- total and c are positive int64 values, so both fit uint64.
		share, remainder := bits.Div64(hi, lo, capacity)
		out[i] = int64(share)       // #nosec G115 -- the proportional share cannot exceed positive int64 total.
		distributed += int64(share) // #nosec G115 -- shares sum to at most positive int64 total.
		rem[i] = remainder
	}
	// Hand out the leftover to the largest remainders first (stable), skipping capped ones.
	leftover := total - distributed
	order := make([]int, 0, len(cap))
	for i := range cap {
		if cap[i] > 0 {
			order = append(order, i)
		}
	}
	sort.SliceStable(order, func(a, b int) bool { return rem[order[a]] > rem[order[b]] })
	// Exact quotient/remainder arithmetic guarantees leftover < len(order).
	// One bounded pass replaces the old potentially unbounded per-cent loop.
	for _, i := range order {
		if leftover == 0 {
			break
		}
		if out[i] < cap[i] {
			out[i]++
			leftover--
		}
	}
	return out
}

// An account count large enough to overflow a uint64 capacity sum is rare.
// Keep ordinary allocation on the fixed-width path, with exact wide arithmetic
// here rather than saturating the denominator and changing the proportions.
func splitProportionalLarge(total int64, cap, out []int64) []int64 {
	capacity := new(big.Int)
	var amount big.Int
	for _, c := range cap {
		capacity.Add(capacity, amount.SetInt64(c))
	}
	rem := make([]big.Int, len(cap))
	order := make([]int, 0, len(cap))
	var distributed int64
	var numerator, quotient big.Int
	for i, c := range cap {
		if c <= 0 {
			continue
		}
		numerator.Mul(amount.SetInt64(total), big.NewInt(c))
		quotient.QuoRem(&numerator, capacity, &rem[i])
		out[i] = quotient.Int64()
		distributed += out[i]
		order = append(order, i)
	}
	sort.SliceStable(order, func(i, j int) bool { return rem[order[i]].Cmp(&rem[order[j]]) > 0 })
	leftover := total - distributed
	for _, i := range order {
		if leftover == 0 {
			break
		}
		if out[i] < cap[i] {
			out[i]++
			leftover--
		}
	}
	return out
}
