// SPDX-License-Identifier: MIT

// Package forecast projects a balance (or net worth) forward over a horizon from
// recurring monthly cash flows plus one-time events. Amounts are integer minor
// units (inflows positive, outflows negative). Pure Go, no platform dependencies;
// unit-tested on native Go. The planning UI calls into here.
package forecast

import (
	"errors"
	"fmt"

	"github.com/monstercameron/CashFlux/internal/money"
)

// MaxProjectionMonths is the checked forecast's maximum horizon (100 years).
const MaxProjectionMonths = 1200

// MaxProjectionWork bounds month/input visits for an interactive projection.
const MaxProjectionWork = 1_000_000

// ErrProjectionBounds means a requested horizon or amount of work exceeds the
// checked model's documented resource limits. No horizon is silently clamped.
var ErrProjectionBounds = errors.New("forecast: projection exceeds model limits")

// CheckProjectionBounds checks a horizon and total number of input flows before
// allocation or iteration. A non-positive horizon performs no month work, but
// the input count remains bounded for helpers that inspect assumptions.
func CheckProjectionBounds(months, inputs int) error {
	if inputs < 0 || inputs >= MaxProjectionWork || months > MaxProjectionMonths {
		return fmt.Errorf("%w: maximum %d months and %d month/input visits", ErrProjectionBounds, MaxProjectionMonths, MaxProjectionWork)
	}
	if months > 0 && inputs+1 > MaxProjectionWork/months {
		return fmt.Errorf("%w: %d months with %d inputs", ErrProjectionBounds, months, inputs)
	}
	return nil
}

func add(a, b int64, checked bool) (int64, error) {
	if checked {
		return money.Add(a, b)
	}
	return a + b, nil
}

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
	net, _ := monthlyNet(recurring, false)
	return net
}

// MonthlyNetChecked sums recurring flows in their original order, reporting
// checked intermediate overflows even when later flows would cancel them.
func MonthlyNetChecked(recurring []Recurring) (int64, error) {
	if err := CheckProjectionBounds(0, len(recurring)); err != nil {
		return 0, err
	}
	return monthlyNet(recurring, true)
}

func monthlyNet(recurring []Recurring, checked bool) (int64, error) {
	var net int64
	for i, r := range recurring {
		var err error
		net, err = add(net, r.Monthly, checked)
		if err != nil {
			return 0, fmt.Errorf("forecast: recurring flow %d (%q): %w", i+1, r.Label, err)
		}
	}
	return net, nil
}

// Project returns the projected end-of-month balance for each of the next
// `months` months, starting from `start`. Each month applies the recurring net
// plus any one-time events scheduled in that month. A non-positive horizon
// yields an empty slice.
func Project(start int64, recurring []Recurring, oneTimes []OneTime, months int) []int64 {
	curve, _ := project(start, recurring, oneTimes, months, false)
	return curve
}

// ProjectChecked runs the same monthly algorithm and rounding-free arithmetic
// as Project, with bounded work and checked additions. Failure returns no curve.
func ProjectChecked(start int64, recurring []Recurring, oneTimes []OneTime, months int) ([]int64, error) {
	if len(recurring) > MaxProjectionWork || len(oneTimes) > MaxProjectionWork {
		return nil, fmt.Errorf("%w: too many inputs", ErrProjectionBounds)
	}
	if err := CheckProjectionBounds(months, len(recurring)+len(oneTimes)); err != nil {
		return nil, err
	}
	return project(start, recurring, oneTimes, months, true)
}

func project(start int64, recurring []Recurring, oneTimes []OneTime, months int, checked bool) ([]int64, error) {
	if months <= 0 {
		return nil, nil
	}
	net, err := monthlyNet(recurring, checked)
	if err != nil {
		return nil, err
	}
	out := make([]int64, months)
	bal := start
	for i := 0; i < months; i++ {
		bal, err = add(bal, net, checked)
		if err != nil {
			return nil, fmt.Errorf("forecast: month %d recurring net: %w", i+1, err)
		}
		for j, o := range oneTimes {
			if o.Month == i+1 {
				bal, err = add(bal, o.Amount, checked)
				if err != nil {
					return nil, fmt.Errorf("forecast: month %d one-time flow %d (%q): %w", i+1, j+1, o.Label, err)
				}
			}
		}
		out[i] = bal
	}
	return out, nil
}
