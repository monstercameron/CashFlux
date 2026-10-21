// SPDX-License-Identifier: MIT

package goalcompare

import (
	"errors"
	"fmt"

	"github.com/monstercameron/CashFlux/internal/money"
)

var (
	// ErrMonthlyPlansUnavailable means at least one goal has no usable monthly plan.
	ErrMonthlyPlansUnavailable = errors.New("goalcompare: monthly plans unavailable")
	// ErrMonthlyCurrencyMismatch means a pooled monthly contribution is not one currency.
	ErrMonthlyCurrencyMismatch = errors.New("goalcompare: monthly plan currencies differ")
)

// MonthlyPool is the checked total contribution available to a two-goal race.
// Amount is meaningful only when Available is true.
type MonthlyPool struct {
	Amount    money.Money
	Available bool
	Err       error
}

// PrepareMonthlyPool combines two known monthly plans without discarding
// currency or overflow errors.
func PrepareMonthlyPool(a, b money.Money, known bool) MonthlyPool {
	if !known {
		return MonthlyPool{Err: ErrMonthlyPlansUnavailable}
	}
	if a.Currency != b.Currency {
		return MonthlyPool{Err: fmt.Errorf("%w: %q and %q", ErrMonthlyCurrencyMismatch, a.Currency, b.Currency)}
	}
	total, err := a.Add(b)
	if err != nil {
		return MonthlyPool{Err: fmt.Errorf("goalcompare: add monthly plans: %w", err)}
	}
	return MonthlyPool{Amount: total, Available: true}
}
