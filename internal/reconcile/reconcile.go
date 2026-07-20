// SPDX-License-Identifier: MIT

// Package reconcile provides pure logic for statement reconciliation: computing
// the difference between an account's cleared balance and a user-supplied
// statement balance, and reporting whether the two agree.
//
// No platform dependencies; unit-tested on native Go.
package reconcile

// Result holds the outcome of a single reconciliation check.
type Result struct {
	// DifferenceMinor is statementMinor − clearedMinor in the account's minor
	// currency units (e.g. cents). Zero means the account is fully reconciled.
	DifferenceMinor int64
	// Reconciled is true when the cleared balance matches the statement balance
	// exactly (difference == 0).
	Reconciled bool
}

// Diff computes the reconciliation result for an account whose cleared balance
// is clearedMinor minor units and whose bank statement shows statementMinor
// minor units. Both values must be in the same currency; callers are responsible
// for the conversion.
func Diff(clearedMinor int64, statementMinor int64) Result {
	diff := statementMinor - clearedMinor
	return Result{DifferenceMinor: diff, Reconciled: diff == 0}
}

// DeltaPreview describes the adjustment that will be posted when the user saves
// an "Update balance" (force-to-target) entry. It is purely informational —
// nothing is written; the caller uses it to render a human-readable preview
// ("current $710.00 → entered $1,115.00 = +$405.00 adjustment") before the
// user confirms.
type DeltaPreview struct {
	// CurrentMinor is the account's present balance in minor units (as computed
	// by the caller from the ledger).
	CurrentMinor int64
	// TargetMinor is the balance the user has typed in.
	TargetMinor int64
	// AdjustmentMinor is (TargetMinor − CurrentMinor). Positive means money is
	// added; negative means it is removed. Zero means no adjustment is needed.
	AdjustmentMinor int64
	// NeedsAdjustment is false when the target already equals the current balance
	// (no transaction will be posted).
	NeedsAdjustment bool
}

// PreviewDelta computes what adjustment (if any) a "set balance to target"
// operation would post.  Both arguments are in the same currency's minor units.
func PreviewDelta(currentMinor int64, targetMinor int64) DeltaPreview {
	adj := targetMinor - currentMinor
	return DeltaPreview{
		CurrentMinor:    currentMinor,
		TargetMinor:     targetMinor,
		AdjustmentMinor: adj,
		NeedsAdjustment: adj != 0,
	}
}

// BulkClear describes the effect of marking a batch of currently-uncleared
// transactions cleared, evaluated against a statement balance. It powers the
// reconcile flow's "Mark all cleared" preview: how many rows would flip, their
// combined amount, the cleared balance that would result, and whether that
// leaves the account reconciled — so the header can promise "this will match the
// statement exactly" or warn about the gap that would remain.
type BulkClear struct {
	// Count is how many transactions would flip from uncleared to cleared.
	Count int
	// SumMinor is the combined signed amount of those transactions in minor
	// units. Clearing a transaction moves its amount into the cleared balance.
	SumMinor int64
	// ProjectedClearedMinor is the cleared balance after all Count transactions
	// are marked cleared (the current cleared balance plus SumMinor).
	ProjectedClearedMinor int64
	// Result is the reconciliation outcome (difference + reconciled flag)
	// measured between the projected cleared balance and the statement balance.
	Result Result
}

// PreviewBulkClear computes what "Mark all cleared" would do: clearing every
// amount in unclearedAmounts on top of the current clearedMinor balance, then
// re-measuring against statementMinor. All values are in the same currency's
// minor units. It mutates nothing; the caller uses it to preview the outcome
// before committing the batch.
func PreviewBulkClear(clearedMinor int64, statementMinor int64, unclearedAmounts []int64) BulkClear {
	var sum int64
	for _, a := range unclearedAmounts {
		sum += a
	}
	projected := clearedMinor + sum
	return BulkClear{
		Count:                 len(unclearedAmounts),
		SumMinor:              sum,
		ProjectedClearedMinor: projected,
		Result:                Diff(projected, statementMinor),
	}
}
