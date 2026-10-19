// SPDX-License-Identifier: MIT

package budgeting

import (
	"fmt"
	"math"

	"github.com/monstercameron/CashFlux/internal/money"
)

// BillFit is the verdict for whether an upcoming charge still fits under a budget
// for the period the charge lands in — the analytical link between the Bills page
// and the Budgets page. All amounts are integer minor units in the budget's limit
// currency; the caller FX-converts the bill before calling.
type BillFit struct {
	// Fits is true when paying the bill keeps period spend at or under the limit.
	Fits bool
	// OverBy is how far paying the bill pushes spend past the limit (0 when it fits).
	OverBy int64
	// LeftAfter is the room remaining under the limit after paying (0 when over).
	LeftAfter int64
}

// FitBill reports whether a charge of `bill` still fits under `limit` given
// `spent` already booked in the same period. Spending exactly to the limit counts
// as fitting (LeftAfter 0, Fits true); a single minor unit past it is over. A
// non-positive limit (an untracked/zero budget) can never be fit — the caller
// should skip showing a chip in that case rather than report a spurious overage.
func FitBill(limit, spent, bill int64) BillFit {
	after, err := money.Add(spent, bill)
	if err != nil {
		return BillFit{Fits: false, OverBy: math.MaxInt64}
	}
	if after > limit {
		overBy, err := money.Sub(after, limit)
		if err != nil {
			return BillFit{Fits: false, OverBy: math.MaxInt64}
		}
		return BillFit{Fits: false, OverBy: overBy}
	}
	leftAfter, err := money.Sub(limit, after)
	if err != nil {
		return BillFit{Fits: false, OverBy: math.MaxInt64}
	}
	return BillFit{Fits: true, LeftAfter: leftAfter}
}

// FitBillChecked reports an error when the resulting fit or overage cannot be
// represented as an int64 minor-unit amount.
func FitBillChecked(limit, spent, bill int64) (BillFit, error) {
	after, err := money.Add(spent, bill)
	if err != nil {
		return BillFit{}, fmt.Errorf("add booked spend and bill: %w", err)
	}
	if after > limit {
		overBy, err := money.Sub(after, limit)
		if err != nil {
			return BillFit{}, fmt.Errorf("calculate bill overage: %w", err)
		}
		return BillFit{OverBy: overBy}, nil
	}
	leftAfter, err := money.Sub(limit, after)
	if err != nil {
		return BillFit{}, fmt.Errorf("calculate remaining room: %w", err)
	}
	return BillFit{Fits: true, LeftAfter: leftAfter}, nil
}
