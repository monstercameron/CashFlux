// SPDX-License-Identifier: MIT

package recurdiscover

import (
	"sort"
	"time"
)

// bandRelTolerance is the fraction of the median a banded amount may deviate by
// before the series is treated as incoherent noise rather than a variable bill.
const bandRelTolerance = 0.35

// noiseRelSpread is the relative MAD above which amounts have no coherent central
// value — the Venmo-payment case that must yield no candidate.
const noiseRelSpread = 0.50

// levelRelTolerance is how close two amounts must be (as a fraction of the larger)
// to count as the same price level when detecting a stepped price change.
const levelRelTolerance = 0.06

// costResult is the outcome of the cost stage for one (already amount-coherent)
// group: the amount model, a 0..1 stability score, and whether the amounts cohere
// at all (false ⇒ reject as noise).
type costResult struct {
	model     AmountModel
	stability float64
	coherent  bool
}

// analyzeCost models a group's amounts. It recognizes a fixed amount, a single
// durable price change (stepped — emitted as a creep signal on the same
// candidate, never a split), or a bounded variable band, and rejects amounts with
// no coherent central value as noise. Dates are parallel to amounts (same order)
// and used only to date a detected price step; callers pass them chronologically.
func analyzeCost(amounts []int64, dates []time.Time) costResult {
	if len(amounts) == 0 {
		return costResult{}
	}
	med := medianInt64(amounts)
	if med == 0 {
		med = amounts[0]
	}

	// Fixed: every amount identical.
	allEqual := true
	for _, a := range amounts {
		if a != amounts[0] {
			allEqual = false
			break
		}
	}
	if allEqual {
		return costResult{
			model:     AmountModel{Kind: AmountFixed, Typical: amounts[0], LowMinor: amounts[0], HighMinor: amounts[0]},
			stability: 1,
			coherent:  true,
		}
	}

	// Stepped: exactly one durable level shift over time.
	if _, from, to, at, ok := detectStep(amounts, dates); ok {
		return costResult{
			model: AmountModel{
				Kind:      AmountStepped,
				Typical:   to,
				LowMinor:  min64(from, to),
				HighMinor: max64(from, to),
				Step:      &PriceStep{FromMinor: from, ToMinor: to, At: at},
			},
			stability: 0.85,
			coherent:  true,
		}
	}

	// Banded vs noise: relative MAD decides coherence.
	mad := madInt64(amounts, med)
	rel := float64(mad) / float64(absInt64(med))
	if rel > noiseRelSpread {
		return costResult{coherent: false}
	}
	lo, hi := med, med
	for _, a := range amounts {
		if a < lo {
			lo = a
		}
		if a > hi {
			hi = a
		}
	}
	tol := mad
	stability := clamp01(1 - rel/bandRelTolerance)
	return costResult{
		model: AmountModel{
			Kind:           AmountBanded,
			Typical:        med,
			LowMinor:       lo,
			HighMinor:      hi,
			ToleranceMinor: tol,
		},
		stability: stability,
		coherent:  true,
	}
}

// detectStep reports a single durable price change: the amounts, taken in date
// order, hold one stable level and then switch to a second stable level and stay
// there (tolerating small within-level wobble). It returns the change if and only
// if there is exactly one such switch. amounts and dates are parallel and assumed
// chronological.
func detectStep(amounts []int64, dates []time.Time) (shifts int, from, to int64, at time.Time, ok bool) {
	if len(amounts) < 4 {
		return 0, 0, 0, time.Time{}, false
	}
	// Count level switches scanning in order; a switch is a move to an amount that
	// differs from the running level by more than the level tolerance.
	level := amounts[0]
	switchIdx := -1
	for i := 1; i < len(amounts); i++ {
		if !sameLevel(amounts[i], level) {
			shifts++
			if shifts > 1 {
				return shifts, 0, 0, time.Time{}, false
			}
			switchIdx = i
			level = amounts[i]
		}
	}
	if shifts != 1 || switchIdx < 0 {
		return shifts, 0, 0, time.Time{}, false
	}
	// Require both sides to have at least two occurrences so a one-off blip is not
	// mistaken for a durable change.
	if switchIdx < 2 || len(amounts)-switchIdx < 2 {
		return shifts, 0, 0, time.Time{}, false
	}
	from = medianInt64(amounts[:switchIdx])
	to = medianInt64(amounts[switchIdx:])
	at = dates[switchIdx]
	return shifts, from, to, at, true
}

// sameLevel reports whether two amounts belong to the same price level (within
// levelRelTolerance of the larger magnitude).
func sameLevel(a, b int64) bool {
	hi := max64(absInt64(a), absInt64(b))
	if hi == 0 {
		return true
	}
	return float64(absInt64(a-b))/float64(hi) <= levelRelTolerance
}

// splitAmounts detects a same-signature cluster that actually holds two
// concurrent subscriptions at two distinct price levels, and returns the two
// index partitions (into the passed, date-ordered txns) when so. It splits only
// when the two amount levels are well separated AND interleave in time (both
// present across the series) — a purely temporal separation is a price step, not
// two subscriptions, and returns nil. txns must be sorted chronologically.
func splitAmounts(txns []Txn, floor int) [][]int {
	if len(txns) < floor*2 {
		return nil
	}
	amounts := make([]int64, len(txns))
	for i, t := range txns {
		amounts[i] = t.AmountMinor
	}
	uniq := append([]int64(nil), amounts...)
	sort.Slice(uniq, func(i, j int) bool { return uniq[i] < uniq[j] })

	// Find the widest relative gap between consecutive distinct amounts.
	bestGap := 0.0
	splitVal := int64(0)
	for i := 1; i < len(uniq); i++ {
		if uniq[i] == uniq[i-1] {
			continue
		}
		mid := float64(uniq[i]+uniq[i-1]) / 2
		if mid == 0 {
			continue
		}
		rel := float64(uniq[i]-uniq[i-1]) / absFloat(mid)
		if rel > bestGap {
			bestGap = rel
			splitVal = (uniq[i] + uniq[i-1]) / 2
		}
	}
	// Require a clear separation between the two levels.
	if bestGap < 0.5 {
		return nil
	}

	var low, high []int
	for i, a := range amounts {
		if a < splitVal {
			low = append(low, i)
		} else {
			high = append(high, i)
		}
	}
	if len(low) < floor || len(high) < floor {
		return nil
	}
	// Each level must be internally coherent (not itself noise).
	if !levelCoherent(amounts, low) || !levelCoherent(amounts, high) {
		return nil
	}
	// Interleave test: both levels must appear in the first and second halves of
	// the series; otherwise it is a temporal step, handled by detectStep.
	half := len(txns) / 2
	if !spansBothHalves(low, half) || !spansBothHalves(high, half) {
		return nil
	}
	return [][]int{low, high}
}

// levelCoherent reports whether the amounts at the given indices cluster tightly
// enough to be one subscription level.
func levelCoherent(amounts []int64, idx []int) bool {
	if len(idx) == 0 {
		return false
	}
	sub := make([]int64, len(idx))
	for i, k := range idx {
		sub[i] = amounts[k]
	}
	med := medianInt64(sub)
	if med == 0 {
		return true
	}
	return float64(madInt64(sub, med))/float64(absInt64(med)) <= bandRelTolerance
}

// spansBothHalves reports whether the index set has at least one member in each
// half of the series [0,half) and [half,n).
func spansBothHalves(idx []int, half int) bool {
	var first, second bool
	for _, k := range idx {
		if k < half {
			first = true
		} else {
			second = true
		}
	}
	return first && second
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
