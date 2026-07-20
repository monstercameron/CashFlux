// SPDX-License-Identifier: MIT

package recurdiscover

import (
	"sort"
	"time"
)

// dayOf truncates a time to its UTC calendar day so mixed-zone dates compare and
// subtract as whole days.
func dayOf(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}

// daysBetween is the whole-day count from a to b (b after a → positive).
func daysBetween(a, b time.Time) int {
	return int(dayOf(b).Sub(dayOf(a)).Hours()) / 24
}

// medianInt returns the median of a copy of xs (0 for empty). For an even count
// it averages the two central values (integer truncation).
func medianInt(xs []int) int {
	if len(xs) == 0 {
		return 0
	}
	s := append([]int(nil), xs...)
	sort.Ints(s)
	n := len(s)
	if n%2 == 1 {
		return s[n/2]
	}
	return (s[n/2-1] + s[n/2]) / 2
}

// medianInt64 is medianInt for int64 (minor-unit amounts).
func medianInt64(xs []int64) int64 {
	if len(xs) == 0 {
		return 0
	}
	s := append([]int64(nil), xs...)
	sort.Slice(s, func(i, j int) bool { return s[i] < s[j] })
	n := len(s)
	if n%2 == 1 {
		return s[n/2]
	}
	return (s[n/2-1] + s[n/2]) / 2
}

// madInt is the median absolute deviation of xs about the given center.
func madInt(xs []int, center int) int {
	if len(xs) == 0 {
		return 0
	}
	dev := make([]int, len(xs))
	for i, x := range xs {
		d := x - center
		if d < 0 {
			d = -d
		}
		dev[i] = d
	}
	return medianInt(dev)
}

// madInt64 is madInt for int64 amounts.
func madInt64(xs []int64, center int64) int64 {
	if len(xs) == 0 {
		return 0
	}
	dev := make([]int64, len(xs))
	for i, x := range xs {
		d := x - center
		if d < 0 {
			d = -d
		}
		dev[i] = d
	}
	return medianInt64(dev)
}

// absInt returns the absolute value of an int.
func absInt(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// absInt64 returns the absolute value of an int64.
func absInt64(x int64) int64 {
	if x < 0 {
		return -x
	}
	return x
}

// clamp01 constrains a float to [0,1].
func clamp01(x float64) float64 {
	if x < 0 {
		return 0
	}
	if x > 1 {
		return 1
	}
	return x
}

// domBuckets clusters a set of day-of-month values into buckets whose members are
// within tol days of the bucket's first member, returning each bucket's center
// (rounded mean) and size, ordered by center. It is the shape test that separates
// semi-monthly (two tight buckets ~two weeks apart) from biweekly (many buckets
// as the anchor precesses through the month).
func domBuckets(doms []int, tol int) []struct {
	Center int
	Size   int
} {
	if len(doms) == 0 {
		return nil
	}
	s := append([]int(nil), doms...)
	sort.Ints(s)
	type bkt struct {
		sum, n, first int
	}
	var bkts []bkt
	for _, dm := range s {
		placed := false
		for i := range bkts {
			if absInt(dm-bkts[i].first) <= tol {
				bkts[i].sum += dm
				bkts[i].n++
				placed = true
				break
			}
		}
		if !placed {
			bkts = append(bkts, bkt{sum: dm, n: 1, first: dm})
		}
	}
	out := make([]struct {
		Center int
		Size   int
	}, len(bkts))
	for i, b := range bkts {
		out[i] = struct {
			Center int
			Size   int
		}{Center: (b.sum + b.n/2) / b.n, Size: b.n}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Center < out[j].Center })
	return out
}
