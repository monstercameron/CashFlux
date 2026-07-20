// SPDX-License-Identifier: MIT

package recurdiscover

import (
	"sort"
	"time"
)

// rhythm is the outcome of the cadence-detection stage for one cluster: the
// detected cadence, a 0..1 fit score (how tightly the gaps hug the cadence), and
// the inferred anchor day + posting window.
type rhythm struct {
	cadence   Cadence
	fit       float64
	anchorDay int
	spread    int
	postsBy   int
}

// minRhythmFit is the fit below which a cluster has no usable rhythm and yields
// no candidate.
const minRhythmFit = 0.30

// detectRhythm infers a cluster's cadence from its occurrence dates. It classifies
// by the median inter-arrival gap, disambiguating the two look-alike pairs by
// day-of-month shape — biweekly (constant ~14-day gap, anchor precesses through
// the month) vs semi-monthly (two fixed anchor days ~two weeks apart), and
// every-4-weeks (28-day gap, day-of-month drifts earlier) vs monthly (day-of-month
// stable) — then scores the fit and derives the anchor + posting window.
func detectRhythm(dates []time.Time) rhythm {
	if len(dates) < 2 {
		return rhythm{cadence: CadenceUnknown}
	}
	ds := append([]time.Time(nil), dates...)
	sort.Slice(ds, func(i, j int) bool { return ds[i].Before(ds[j]) })

	gaps := make([]int, 0, len(ds)-1)
	for i := 1; i < len(ds); i++ {
		gaps = append(gaps, daysBetween(ds[i-1], ds[i]))
	}
	med := medianInt(gaps)
	mad := madInt(gaps, med)

	var cad Cadence
	switch {
	case med < 10:
		cad = CadenceWeekly
	case med < 20:
		cad = disambiguateBiweeklySemimonthly(ds, med, mad)
	case med < 45:
		cad = disambiguateMonthlyFourWeekly(ds, med, mad)
	case med < 135:
		cad = CadenceQuarterly
	case med < 270:
		cad = CadenceSemiannual
	default:
		cad = CadenceAnnual
	}

	fit := gapFit(cad, float64(med), float64(mad))
	anchor, spread, postsBy := anchorWindow(cad, ds)
	return rhythm{cadence: cad, fit: fit, anchorDay: anchor, spread: spread, postsBy: postsBy}
}

// disambiguateBiweeklySemimonthly decides between a constant ~14-day rhythm and a
// twice-a-month rhythm. Semi-monthly occurrences fall on two stable day-of-month
// anchors ~two weeks apart; biweekly occurrences walk around the month, producing
// many day-of-month buckets. A near-constant 14-day gap with more than two anchor
// buckets reads as biweekly.
func disambiguateBiweeklySemimonthly(ds []time.Time, med, mad int) Cadence {
	doms := domsOf(ds)
	buckets := domBuckets(doms, 2)
	if len(buckets) <= 2 {
		// Two (or one) fixed anchor days ⇒ semi-monthly, provided the two anchors
		// sit roughly two weeks apart (or all land on one day, degenerate monthly-
		// ish but at a ~15-day gap we still read it as semi-monthly).
		if len(buckets) == 2 {
			sep := absInt(buckets[1].Center - buckets[0].Center)
			if sep >= 10 && sep <= 20 {
				return CadenceSemimonthly
			}
		} else {
			return CadenceSemimonthly
		}
	}
	// Many anchor days with a tight ~14-day gap ⇒ biweekly.
	if med <= 15 && mad <= 2 {
		return CadenceBiweekly
	}
	return CadenceSemimonthly
}

// disambiguateMonthlyFourWeekly decides between a day-of-month-stable monthly
// rhythm and a 28-day every-4-weeks rhythm whose day-of-month drifts earlier each
// cycle. A tightly clustered day-of-month reads as monthly; otherwise a median gap
// at or under 29 days reads as every-4-weeks.
func disambiguateMonthlyFourWeekly(ds []time.Time, med, mad int) Cadence {
	doms := domsOf(ds)
	buckets := domBuckets(doms, 3)
	// One tight day-of-month bucket covering the bulk of occurrences ⇒ monthly.
	if len(buckets) == 1 {
		return CadenceMonthly
	}
	if med <= 29 {
		return CadenceEvery4Weeks
	}
	return CadenceMonthly
}

// gapFit scores how tightly the observed gaps match the cadence's nominal gap: it
// penalizes both the median's deviation from nominal and the gap jitter (MAD),
// each normalized by the nominal gap. Result is clamped to [0,1].
func gapFit(c Cadence, med, mad float64) float64 {
	nominal := c.nominalGap()
	if nominal <= 0 {
		return 0
	}
	err := absFloat(med-nominal) / nominal
	jitter := mad / nominal
	return clamp01(1 - err - jitter)
}

func absFloat(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}

// anchorWindow infers the anchor day and posting window for a cadence. The
// monthly family (monthly, semi-monthly, quarterly, semiannual, annual) anchors on
// a day-of-month; the weekly family (weekly, biweekly, every-4-weeks) anchors on
// an ISO weekday. The spread is how many days later than the anchor the latest
// occurrences drift (processing lag / weekend shift), and PostsBy = anchor +
// spread.
func anchorWindow(c Cadence, ds []time.Time) (anchor, spread, postsBy int) {
	switch c {
	case CadenceWeekly, CadenceBiweekly, CadenceEvery4Weeks:
		wds := make([]int, len(ds))
		for i, t := range ds {
			wds[i] = int(dayOf(t).Weekday()) // 0=Sun..6=Sat
		}
		anchor = mostCommonInt(wds)
		spread = 0
		for _, w := range wds {
			if diff := w - anchor; diff > spread {
				spread = diff
			}
		}
		postsBy = anchor + spread
		// Present weekday as ISO 1..7 (Mon..Sun) for the UI.
		anchor = isoWeekday(anchor)
		postsBy = isoWeekday(postsBy % 7)
		return anchor, spread, postsBy
	default:
		doms := domsOf(ds)
		anchor = medianInt(doms)
		spread = 0
		for _, dm := range doms {
			if diff := dm - anchor; diff > spread {
				spread = diff
			}
		}
		if c == CadenceSemimonthly {
			// Anchor on the earlier of the two twice-monthly days for a stable label.
			if b := domBuckets(doms, 2); len(b) >= 1 {
				anchor = b[0].Center
				spread = 0
			}
		}
		postsBy = anchor + spread
		return anchor, spread, postsBy
	}
}

// domsOf extracts the day-of-month of each date.
func domsOf(ds []time.Time) []int {
	out := make([]int, len(ds))
	for i, t := range ds {
		out[i] = t.Day()
	}
	return out
}

// mostCommonInt returns the most frequent value, breaking ties by the smallest
// value for determinism.
func mostCommonInt(xs []int) int {
	counts := map[int]int{}
	for _, x := range xs {
		counts[x]++
	}
	best, bestN := 0, -1
	for v, n := range counts {
		if n > bestN || (n == bestN && v < best) {
			best, bestN = v, n
		}
	}
	return best
}

// isoWeekday converts a Go weekday (0=Sun..6=Sat) to ISO (1=Mon..7=Sun).
func isoWeekday(goWeekday int) int {
	if goWeekday == 0 {
		return 7
	}
	return goWeekday
}
