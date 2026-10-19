// SPDX-License-Identifier: MIT

package split

import (
	"errors"

	"github.com/monstercameron/CashFlux/internal/money"
)

var (
	// ErrUnbalanced means the supplied values do not reconcile to the target.
	ErrUnbalanced = errors.New("split values do not balance")
	// ErrNonPositive means a split line has no positive value.
	ErrNonPositive = errors.New("split values must be positive")
	// ErrTooSmall means a percentage share rounds to zero minor units.
	ErrTooSmall = errors.New("percentage share rounds to zero")
)

// Balanced reports whether nonnegative values sum to target without overflow.
func Balanced(target int64, values []int64) bool {
	sum, err := money.SumMinor(values...)
	return err == nil && sum == target
}

// Reconcile validates amount or percentage values and returns the amount for
// each line. Percent values are basis points and use stable largest remainder
// allocation. The input slice is never modified.
func Reconcile(total int64, values []int64, percent bool) ([]int64, error) {
	if total < 0 {
		return nil, ErrUnbalanced
	}
	for _, value := range values {
		if value <= 0 {
			return nil, ErrNonPositive
		}
	}
	if percent {
		if !Balanced(PercentScale, values) {
			return nil, ErrUnbalanced
		}
		amounts, err := money.Allocate(total, values)
		if err != nil {
			return nil, errors.Join(ErrUnbalanced, err)
		}
		for _, amount := range amounts {
			if amount == 0 {
				return nil, ErrTooSmall
			}
		}
		return amounts, nil
	}
	if !Balanced(total, values) {
		return nil, ErrUnbalanced
	}
	return append([]int64(nil), values...), nil
}
