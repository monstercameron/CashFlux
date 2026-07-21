// SPDX-License-Identifier: MIT

package payoff

import (
	"math"
	"sort"
)

// Debt is one liability in a multi-debt payoff plan. Balance is the positive
// amount owed and MinPayment the required monthly minimum, both in minor units;
// AprPercent is the annual interest rate (e.g. 19.99).
type Debt struct {
	Name       string
	Balance    int64
	AprPercent float64
	MinPayment int64
}

// Strategy chooses which debt receives the extra payment first.
type Strategy int

const (
	// Snowball targets the smallest balance first — quick wins for momentum.
	Snowball Strategy = iota
	// Avalanche targets the highest APR first — the least total interest.
	Avalanche
)

// Plan is the outcome of a multi-debt payoff simulation.
type Plan struct {
	Months        int      // months until every debt is cleared
	TotalInterest int64    // total interest paid across all debts, minor units
	TotalPaid     int64    // principal + interest paid, minor units
	Order         []string // debt names in the order they were paid off
	ClearedMonths []int    // 1-based month each Order entry was paid off (parallel to Order)
	Schedule      []int64  // remaining total balance (minor units) at the end of each month; len == Months, last is 0
}

// BuildPlan simulates clearing several debts together using the classic debt
// snowball/avalanche method: every month each debt accrues interest and is paid
// its minimum, then all remaining firepower (the extra plus the minimums freed by
// already-cleared debts) is thrown at one focus debt chosen by strategy. When the
// focus clears mid-month the leftover cascades to the next focus.
//
// The monthly budget is the sum of every debt's minimum plus extra, held constant
// as debts clear (that constancy is what accelerates payoff). It returns ok=false
// when extra is negative, the budget is non-positive, or the budget can't make
// progress in some month (total balance fails to fall) so the debts would never
// clear. No debts owed is ok=true with a zero plan.
func BuildPlan(debts []Debt, extra int64, strategy Strategy) (Plan, bool) {
	if extra < 0 {
		return Plan{}, false
	}
	n := len(debts)
	bal := make([]int64, n)
	cleared := make([]bool, n)
	var budget int64
	active := 0
	for i, d := range debts {
		if d.Balance > 0 {
			bal[i] = d.Balance
			active++
		} else {
			cleared[i] = true // nothing owed → already done, never recorded in Order
		}
		if d.MinPayment > 0 {
			budget += d.MinPayment
		}
	}
	budget += extra
	if active == 0 {
		return Plan{}, true
	}
	if budget <= 0 {
		return Plan{}, false
	}

	var totalInterest, totalPaid int64
	var order []string
	var clearedMonths []int
	var schedule []int64
	months := 0

	for months < maxMonths {
		var sumBefore int64
		for _, b := range bal {
			if b > 0 {
				sumBefore += b
			}
		}
		if sumBefore == 0 {
			break // all cleared
		}

		// Accrue interest on every active debt.
		for i := range bal {
			if bal[i] <= 0 {
				continue
			}
			interest := int64(math.Round(float64(bal[i]) * debts[i].AprPercent / 1200.0))
			if interest < 0 {
				interest = 0
			}
			bal[i] += interest
			totalInterest += interest
		}

		avail := budget
		// Pay each active debt its minimum (capped at balance and remaining budget).
		for i := range bal {
			if bal[i] <= 0 || avail <= 0 {
				continue
			}
			pay := debts[i].MinPayment
			if pay < 0 {
				pay = 0
			}
			if pay > bal[i] {
				pay = bal[i]
			}
			if pay > avail {
				pay = avail
			}
			bal[i] -= pay
			avail -= pay
			totalPaid += pay
		}
		// Throw the rest at the focus debt, cascading as debts clear.
		for avail > 0 {
			fi := focusIndex(debts, bal, strategy)
			if fi < 0 {
				break
			}
			pay := avail
			if pay > bal[fi] {
				pay = bal[fi]
			}
			bal[fi] -= pay
			avail -= pay
			totalPaid += pay
		}

		// Record debts that cleared this month (index order).
		var sumAfter int64
		for i := range bal {
			if bal[i] > 0 {
				sumAfter += bal[i]
				continue
			}
			if !cleared[i] {
				cleared[i] = true
				order = append(order, debts[i].Name)
				clearedMonths = append(clearedMonths, months+1) // 1-based month it cleared
			}
		}
		schedule = append(schedule, sumAfter) // remaining total balance after this month
		months++

		// No progress (budget couldn't outpace interest) → never clears.
		if sumAfter > 0 && sumAfter >= sumBefore {
			return Plan{}, false
		}
	}

	// Still owing after the cap → not viable.
	for _, b := range bal {
		if b > 0 {
			return Plan{}, false
		}
	}
	return Plan{Months: months, TotalInterest: totalInterest, TotalPaid: totalPaid, Order: order, ClearedMonths: clearedMonths, Schedule: schedule}, true
}

// FocusOrder returns the debt names in the order the strategy attacks them — the
// order that answers "which do I pay first?" and drives the payoff ladder. This is
// deliberately NOT the order debts happen to clear (a tiny low-rate debt can clear
// first on its minimum alone), which would make an avalanche ladder look unsorted.
//
// Avalanche attacks the highest APR first (ties → the smaller balance, so a quick
// win breaks the tie); snowball attacks the smallest balance first (ties → the
// higher APR). Debts with a non-positive balance are dropped. The input slice is
// not mutated.
func FocusOrder(debts []Debt, strategy Strategy) []string {
	ordered := make([]Debt, 0, len(debts))
	for _, d := range debts {
		if d.Balance > 0 {
			ordered = append(ordered, d)
		}
	}
	sort.SliceStable(ordered, func(i, j int) bool {
		a, b := ordered[i], ordered[j]
		if strategy == Avalanche {
			if a.AprPercent != b.AprPercent {
				return a.AprPercent > b.AprPercent
			}
			return a.Balance < b.Balance
		}
		// Snowball.
		if a.Balance != b.Balance {
			return a.Balance < b.Balance
		}
		return a.AprPercent > b.AprPercent
	})
	names := make([]string, len(ordered))
	for i, d := range ordered {
		names[i] = d.Name
	}
	return names
}

// focusIndex returns the index of the active debt the strategy targets next, or
// -1 when none remain. Snowball picks the smallest balance, avalanche the highest
// APR; ties resolve to the lowest index for determinism.
func focusIndex(debts []Debt, bal []int64, strategy Strategy) int {
	best := -1
	for i := range bal {
		if bal[i] <= 0 {
			continue
		}
		if best < 0 {
			best = i
			continue
		}
		switch strategy {
		case Avalanche:
			if debts[i].AprPercent > debts[best].AprPercent {
				best = i
			}
		default: // Snowball
			if bal[i] < bal[best] {
				best = i
			}
		}
	}
	return best
}
