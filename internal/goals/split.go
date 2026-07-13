// SPDX-License-Identifier: MIT

package goals

import "sort"

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
	var capacity int64
	for i, a := range avail {
		if a > 0 {
			cap[i] = a
			capacity += a
		}
	}
	if capacity <= 0 {
		return out
	}
	if total > capacity {
		total = capacity
	}
	if mode == SplitProportional {
		return splitProportional(total, cap, out)
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
func splitProportional(total int64, cap []int64, out []int64) []int64 {
	var capacity int64
	for _, c := range cap {
		capacity += c
	}
	if capacity <= 0 {
		return out
	}
	rem := make([]int64, len(cap))
	var distributed int64
	for i, c := range cap {
		if c <= 0 {
			continue
		}
		num := total * c
		share := num / capacity
		if share > c {
			share = c
		}
		out[i] = share
		distributed += share
		rem[i] = num % capacity
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
	for leftover > 0 {
		progressed := false
		for _, i := range order {
			if leftover == 0 {
				break
			}
			if out[i] < cap[i] {
				out[i]++
				leftover--
				progressed = true
			}
		}
		if !progressed {
			break
		}
	}
	return out
}
