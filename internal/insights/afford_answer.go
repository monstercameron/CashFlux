// SPDX-License-Identifier: MIT

// Package insights — afford_answer.go
// AffordAnswer produces a deterministic, explainable affordability result from
// the user's real figures. Pure Go; no syscall/js.
package insights

import (
	"fmt"

	"github.com/monstercameron/CashFlux/internal/afford"
)

// AffordResult is the explainable outcome returned to the caller (UI layer or
// tests). Projected is in minor units; Surplus is positive when affordable and
// negative when there is a shortfall.
type AffordResult struct {
	CanAfford   bool
	Projected   int64    // projected balance at MonthsAhead (minor units)
	Available   int64    // Projected minus reserve (minor units)
	Surplus     int64    // Available − Amount (negative = shortfall)
	Assumptions []string // plain-English explanation of each input used
}

// defaultMonthsAhead is the fallback horizon when the query has no target date.
const defaultMonthsAhead = 1

// AffordAnswer derives an affordability answer from first principles using the
// user's current balance, their trailing monthly net cash flow, and a fractional
// reserve the caller asks to keep aside (e.g. 0.10 = keep 10 % of balance in
// reserve). All money values are integer minor units (cents).
//
// The reserve is computed as balance * reservePct, floor-truncated to a whole
// number of minor units. If reservePct ≤ 0 no reserve is applied.
//
// MonthsAhead defaults to defaultMonthsAhead (1) when the query carries 0.
func AffordAnswer(q AffordQuery, balance, monthlyNet int64, reservePct float64) AffordResult {
	months := q.MonthsAhead
	if months == 0 {
		months = defaultMonthsAhead
	}
	var reserved int64
	if reservePct > 0 && balance > 0 {
		reserved = int64(float64(balance) * reservePct)
	}

	r := afford.CanAfford(q.Amount, balance, monthlyNet, months, reserved)

	surplus := r.Available - q.Amount

	assumptions := buildAssumptions(balance, monthlyNet, months, reserved, q)

	return AffordResult{
		CanAfford:   r.Affordable,
		Projected:   r.ProjectedBalance,
		Available:   r.Available,
		Surplus:     surplus,
		Assumptions: assumptions,
	}
}

// buildAssumptions returns a plain-English list of the inputs used so the
// answer is transparent and auditable by the user.
func buildAssumptions(balance, monthlyNet int64, months int, reserved int64, q AffordQuery) []string {
	a := []string{
		fmt.Sprintf("Current balance: %s", fmtMinorUnits(balance)),
		fmt.Sprintf("Monthly net cash flow: %s", fmtMinorUnits(monthlyNet)),
		fmt.Sprintf("Horizon: %d month(s)", months),
	}
	if reserved != 0 {
		a = append(a, fmt.Sprintf("Reserve kept aside: %s", fmtMinorUnits(reserved)))
	}
	if q.TargetLabel != "" {
		a = append(a, fmt.Sprintf("Target: %s", q.TargetLabel))
	}
	return a
}

// fmtMinorUnits formats an integer minor-unit amount as a dollar string
// (e.g. 120000 → "$1,200.00"). This is the pure-logic layer's formatter; the
// UI layer uses the richer money.Format function with FX tables.
func fmtMinorUnits(v int64) string {
	neg := v < 0
	if neg {
		v = -v
	}
	dollars := v / 100
	cents := v % 100
	// Insert thousands separators
	s := fmt.Sprintf("%d", dollars)
	if len(s) > 3 {
		out := make([]byte, 0, len(s)+len(s)/3)
		mod := len(s) % 3
		for i, c := range []byte(s) {
			if i > 0 && (i-mod)%3 == 0 {
				out = append(out, ',')
			}
			out = append(out, c)
		}
		s = string(out)
	}
	result := fmt.Sprintf("$%s.%02d", s, cents)
	if neg {
		result = "-" + result
	}
	return result
}
