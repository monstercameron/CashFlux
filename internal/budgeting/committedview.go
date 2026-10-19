// SPDX-License-Identifier: MIT

package budgeting

// PriorSpendComparison describes prior spending against the current effective
// limit, including a bounded progress fill and signed distance from the limit.
type PriorSpendComparison struct {
	Percent int
	Fill    int
	Delta   int64 // positive means under the limit; negative means over
}

// ComparePriorSpend computes the view values for the last-period overlay.
func ComparePriorSpend(priorSpent, effectiveLimit int64) PriorSpendComparison {
	result := PriorSpendComparison{Delta: effectiveLimit - priorSpent}
	if effectiveLimit <= 0 {
		return result
	}
	result.Percent = int(priorSpent * 100 / effectiveLimit)
	if result.Percent < 0 {
		result.Percent = 0
	}
	result.Fill = result.Percent
	if result.Fill > 100 {
		result.Fill = 100
	}
	return result
}

// CommittedPercentages returns the spent and committed portions of a budget
// limit, each as a whole percentage. Committed is clipped to the unspent share
// so the progress segments never exceed the limit.
func CommittedPercentages(limit, spent, committed int64) (spentPercent, committedPercent int) {
	if limit > 0 {
		spentPercent = int(spent * 100 / limit)
		committedPercent = int(committed * 100 / limit)
	}
	if spentPercent > 100 {
		spentPercent = 100
	}
	if spentPercent+committedPercent > 100 {
		committedPercent = 100 - spentPercent
	}
	if committedPercent < 0 {
		committedPercent = 0
	}
	return spentPercent, committedPercent
}
