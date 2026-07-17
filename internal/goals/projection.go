// SPDX-License-Identifier: MIT

package goals

import "time"

// projectionCapMonths bounds the compounding search so a goal that will never be
// reached (contributions + growth too small) terminates instead of looping — 1200
// months is 100 years, far beyond any real horizon.
const projectionCapMonths = 1200

// GrowthProjection is the outcome of compounding a goal's balance at an expected
// annual return, plus its monthly contribution, until it reaches its target.
type GrowthProjection struct {
	// Months until the target is reached (0 = already at/over target).
	Months int
	// Date is the projected completion date (from + Months).
	Date time.Time
	// Reachable is false when contributions + growth never reach the target within
	// the cap (e.g. no contribution and no growth on an under-target balance).
	Reachable bool
	// MonthsNoGrowth is the same projection with 0% return — so the UI can show how
	// much sooner the expected return gets there (the compounding benefit).
	MonthsNoGrowth int
}

// ProjectWithGrowth projects when a goal reaches its target by compounding its
// current balance at annualBips (basis points, e.g. 700 = 7.00% APR) monthly and
// adding monthlyMinor each month. All amounts are base-currency minor units. It is
// pure and deterministic — the return rate is the user's own assumption, nothing is
// fetched. A zero annualBips degrades to a plain no-growth pace projection.
func ProjectWithGrowth(currentMinor, targetMinor, monthlyMinor int64, annualBips int, from time.Time) GrowthProjection {
	noGrowth := monthsToTarget(currentMinor, targetMinor, monthlyMinor, 0)
	if currentMinor >= targetMinor {
		return GrowthProjection{Months: 0, Date: from, Reachable: true, MonthsNoGrowth: 0}
	}
	n := monthsToTarget(currentMinor, targetMinor, monthlyMinor, annualBips)
	if n < 0 {
		return GrowthProjection{Reachable: false, MonthsNoGrowth: clampMonths(noGrowth)}
	}
	return GrowthProjection{
		Months:         n,
		Date:           from.AddDate(0, n, 0),
		Reachable:      true,
		MonthsNoGrowth: clampMonths(noGrowth),
	}
}

// monthsToTarget returns the number of monthly steps for a balance starting at
// currentMinor, growing at annualBips/12 each month and receiving monthlyMinor,
// to reach targetMinor. Returns -1 when it never reaches the target within the cap.
func monthsToTarget(currentMinor, targetMinor, monthlyMinor int64, annualBips int) int {
	if currentMinor >= targetMinor {
		return 0
	}
	r := float64(annualBips) / 10000.0 / 12.0
	bal := float64(currentMinor)
	target := float64(targetMinor)
	contrib := float64(monthlyMinor)
	for i := 1; i <= projectionCapMonths; i++ {
		bal = bal*(1+r) + contrib
		if bal >= target {
			return i
		}
	}
	return -1
}

// clampMonths maps a -1 (unreachable) to 0 so MonthsNoGrowth is a plain non-negative
// count the UI can compare against; callers use Reachable for the real signal.
func clampMonths(n int) int {
	if n < 0 {
		return 0
	}
	return n
}
