// SPDX-License-Identifier: MIT

// Package idlecash prices the opportunity cost of cash sitting idle in low- or
// no-yield accounts (AC15) — the honest, local-first version of the cash-drag
// argument. It is the mirror of the liability carrying-cost figure: carrying cost
// prices debt, idle cash prices the money you *could* be earning on.
//
// The model is deliberately simple and explainable:
//
//	idle     = max(0, liquid − committed)
//	forgone  = idle × benchmark/100   (per year)
//
// where liquid is spendable cash across cash-type accounts, committed is what the
// near-term bills, goal set-asides, and budgets already claim (from billsched /
// safespend — the caller supplies it so this package stays pure), and benchmark is
// a USER-ENTERED annual rate. There are no live feeds and no market assumptions:
// the copy states the benchmark came from the user. Money is int64 minor units.
//
// Pure Go, no syscall/js, no I/O. Unit-testable on native Go.
package idlecash

// DefaultThresholdMinor is the idle amount below which Evaluate does not raise the
// flag: a small buffer over committed spending is prudent, not drag. $500 by
// default (50000 cents). Callers may override via Inputs.ThresholdMinor.
const DefaultThresholdMinor = int64(50000)

// Inputs are the figures the idle-cash calculation needs, all in the same base
// currency's minor units. The caller computes LiquidMinor and CommittedMinor from
// the shared balance map + billsched/safespend so this package carries no domain
// dependencies.
type Inputs struct {
	// LiquidMinor is spendable cash across cash-type accounts (base minor units).
	LiquidMinor int64

	// CommittedMinor is what near-term obligations already claim — bills due, goal
	// set-asides, and committed budgets (base minor units). Negative values are
	// treated as zero.
	CommittedMinor int64

	// BenchmarkAPRPercent is the user-entered annual yield they compare against
	// (e.g. 4.5 for a 4.5% high-yield savings account). Zero disables the forgone
	// figure (nothing to compare to); negative is treated as zero.
	BenchmarkAPRPercent float64

	// ThresholdMinor is the minimum idle amount that raises the flag. Zero uses
	// DefaultThresholdMinor; a negative value means "flag any positive idle".
	ThresholdMinor int64
}

// Result is the explainable idle-cash breakdown.
type Result struct {
	// LiquidMinor and CommittedMinor echo the inputs (committed floored at zero) so
	// the breakdown is self-contained for the UI.
	LiquidMinor    int64
	CommittedMinor int64

	// IdleMinor is max(0, liquid − committed): cash beyond what near-term
	// obligations claim.
	IdleMinor int64

	// ForgoneAnnualMinor is the yearly yield left on the table: idle × benchmark/100,
	// truncated to whole minor units. Zero when the benchmark is zero.
	ForgoneAnnualMinor int64

	// ForgoneMonthlyMinor is ForgoneAnnualMinor ÷ 12 (truncated) — the monthly framing
	// that lines up with the carrying-cost figure.
	ForgoneMonthlyMinor int64

	// BenchmarkAPRPercent echoes the user-entered rate the forgone figure used, so
	// the copy can state the assumption ("at your 4.5% benchmark").
	BenchmarkAPRPercent float64

	// Flag is true when there is meaningful idle cash to act on: idle ≥ threshold
	// AND a positive benchmark to make the opportunity concrete.
	Flag bool
}

// Evaluate runs the idle-cash model over in and returns an explainable Result.
func Evaluate(in Inputs) Result {
	committed := in.CommittedMinor
	if committed < 0 {
		committed = 0
	}
	idle := in.LiquidMinor - committed
	if idle < 0 {
		idle = 0
	}

	bench := in.BenchmarkAPRPercent
	if bench < 0 {
		bench = 0
	}

	// forgone = idle × benchmark/100, computed in float then truncated. idle is
	// minor units; the product stays in minor units.
	forgoneAnnual := int64(float64(idle) * bench / 100)
	forgoneMonthly := forgoneAnnual / 12

	threshold := in.ThresholdMinor
	if threshold == 0 {
		threshold = DefaultThresholdMinor
	}
	if threshold < 0 {
		threshold = 0
	}

	return Result{
		LiquidMinor:         in.LiquidMinor,
		CommittedMinor:      committed,
		IdleMinor:           idle,
		ForgoneAnnualMinor:  forgoneAnnual,
		ForgoneMonthlyMinor: forgoneMonthly,
		BenchmarkAPRPercent: bench,
		Flag:                idle >= threshold && bench > 0,
	}
}
