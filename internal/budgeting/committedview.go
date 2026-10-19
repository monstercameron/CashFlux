// SPDX-License-Identifier: MIT

package budgeting

import (
	"fmt"

	"github.com/monstercameron/CashFlux/internal/money"
)

// PriorSpendComparison describes prior spending against the current effective
// limit, including a bounded progress fill and signed distance from the limit.
type PriorSpendComparison struct {
	Percent int
	Fill    int
	Delta   int64 // positive means under the limit; negative means over
}

// ComparePriorSpend computes the view values for the last-period overlay.
func ComparePriorSpend(priorSpent, effectiveLimit int64) PriorSpendComparison {
	result, err := ComparePriorSpendChecked(priorSpent, effectiveLimit)
	if err != nil {
		return PriorSpendComparison{}
	}
	return result
}

// ComparePriorSpendChecked computes the overlay values and reports overflow.
func ComparePriorSpendChecked(priorSpent, effectiveLimit int64) (PriorSpendComparison, error) {
	delta, err := money.Sub(effectiveLimit, priorSpent)
	if err != nil {
		return PriorSpendComparison{}, fmt.Errorf("compare prior spend: %w", err)
	}
	result := PriorSpendComparison{Delta: delta}
	if effectiveLimit <= 0 {
		return result, nil
	}
	percent, err := money.MulDiv(priorSpent, 100, effectiveLimit, money.RoundTowardZero)
	if err != nil {
		return PriorSpendComparison{}, fmt.Errorf("calculate prior spend percentage: %w", err)
	}
	if percent > int64(^uint(0)>>1) || percent < -int64(^uint(0)>>1)-1 {
		return PriorSpendComparison{}, fmt.Errorf("prior spend percentage does not fit int")
	}
	result.Percent = int(percent)
	if result.Percent < 0 {
		result.Percent = 0
	}
	result.Fill = result.Percent
	if result.Fill > 100 {
		result.Fill = 100
	}
	return result, nil
}

// CommittedPercentages returns the spent and committed portions of a budget
// limit, each as a whole percentage. Committed is clipped to the unspent share
// so the progress segments never exceed the limit.
func CommittedPercentages(limit, spent, committed int64) (spentPercent, committedPercent int) {
	spentPercent, committedPercent, _ = CommittedPercentagesChecked(limit, spent, committed)
	return spentPercent, committedPercent
}

// CommittedPercentagesChecked computes progress segments and reports overflow.
func CommittedPercentagesChecked(limit, spent, committed int64) (spentPercent, committedPercent int, err error) {
	if limit > 0 {
		spentValue, calcErr := money.MulDiv(spent, 100, limit, money.RoundTowardZero)
		if calcErr != nil {
			return 0, 0, fmt.Errorf("calculate spent percentage: %w", calcErr)
		}
		committedValue, calcErr := money.MulDiv(committed, 100, limit, money.RoundTowardZero)
		if calcErr != nil {
			return 0, 0, fmt.Errorf("calculate committed percentage: %w", calcErr)
		}
		maxInt := int64(^uint(0) >> 1)
		if spentValue > maxInt || spentValue < -maxInt-1 || committedValue > maxInt || committedValue < -maxInt-1 {
			return 0, 0, fmt.Errorf("progress percentage does not fit int")
		}
		spentPercent, committedPercent = int(spentValue), int(committedValue)
	}
	if spentPercent > 100 {
		spentPercent = 100
	}
	if committedPercent > 100-spentPercent {
		committedPercent = 100 - spentPercent
	}
	if committedPercent < 0 {
		committedPercent = 0
	}
	return spentPercent, committedPercent, nil
}
