// SPDX-License-Identifier: MIT

// Package forecast projects a balance (or net worth) forward over a horizon from
// recurring monthly cash flows plus one-time events. Amounts are integer minor
// units (inflows positive, outflows negative). Pure Go, no platform dependencies;
// unit-tested on native Go. The planning UI calls into here.
package forecast

// Recurring is a cash flow that repeats every month.
type Recurring struct {
	Label   string
	Monthly int64 // minor units; positive = inflow, negative = outflow
}

// OneTime is a single cash flow at a specific month in the horizon (1-based: 1 is
// the first projected month).
type OneTime struct {
	Label  string
	Month  int
	Amount int64 // minor units; positive = inflow, negative = outflow
}

// MonthlyNet sums all recurring monthly flows (the steady monthly change).
func MonthlyNet(recurring []Recurring) int64 {
	var net int64
	for _, r := range recurring {
		net += r.Monthly
	}
	return net
}

// Project returns the projected end-of-month balance for each of the next
// `months` months, starting from `start`. Each month applies the recurring net
// plus any one-time events scheduled in that month. A non-positive horizon
// yields an empty slice.
func Project(start int64, recurring []Recurring, oneTimes []OneTime, months int) []int64 {
	if months <= 0 {
		return nil
	}
	net := MonthlyNet(recurring)
	out := make([]int64, months)
	bal := start
	for i := 0; i < months; i++ {
		bal += net
		for _, o := range oneTimes {
			if o.Month == i+1 {
				bal += o.Amount
			}
		}
		out[i] = bal
	}
	return out
}
