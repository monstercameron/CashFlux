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
