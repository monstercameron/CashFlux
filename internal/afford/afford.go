// SPDX-License-Identifier: MIT

// Package afford answers "can we afford $X by a target date?" deterministically,
// so an affordability question is backed by the user's own projected cash flow
// (the determinism/explainability rule) rather than an LLM guess. The inputs are a
// starting liquid balance, a steady monthly net cash flow, the number of months
// until the target date, and an amount to keep reserved (existing commitments, a
// safety buffer, and/or goal contributions).
//
// All amounts are integer minor units. Pure Go, no platform dependencies; unit-
// tested on native Go.
package afford

// Result is the explainable outcome of an affordability check.
type Result struct {
	Affordable       bool  // the amount fits within Available by the target date
	ProjectedBalance int64 // balance projected to the target month (minor units)
	Available        int64 // ProjectedBalance minus what's reserved (free to spend)
	Shortfall        int64 // how much short, when not affordable (0 otherwise)
	MonthsNeeded     int   // months until affordable at this rate: 0 if already, -1 if never
}

// CanAfford projects the balance to the target month and reports whether amount
// fits within what's free after reserved. months is the count until the target
// date (0 = now); a negative months is treated as 0.
func CanAfford(amount, start, monthlyNet int64, months int, reserved int64) Result {
	if months < 0 {
		months = 0
	}
	projected := start + monthlyNet*int64(months)
	available := projected - reserved
	res := Result{
		Affordable:       available >= amount,
		ProjectedBalance: projected,
		Available:        available,
		MonthsNeeded:     monthsUntilAffordable(amount, start, monthlyNet, reserved),
	}
	if !res.Affordable {
		res.Shortfall = amount - available
	}
	return res
}

// monthsUntilAffordable returns the first month m where
// start + monthlyNet*m - reserved >= amount: 0 if already affordable, -1 when a
// non-positive cash flow means the target is never reached.
func monthsUntilAffordable(amount, start, monthlyNet, reserved int64) int {
	need := amount + reserved - start
	if need <= 0 {
		return 0
	}
	if monthlyNet <= 0 {
		return -1
	}
	m := need / monthlyNet
	if need%monthlyNet != 0 {
		m++
	}
	return int(m)
}
