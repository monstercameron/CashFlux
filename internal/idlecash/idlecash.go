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

import (
	"math"
	"math/big"
	"strconv"
)

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
	// figure (nothing to compare to); negative, NaN and infinite values are
	// treated as zero.
	BenchmarkAPRPercent float64

	// ThresholdMinor is the minimum idle amount that raises the flag. Zero uses
	// DefaultThresholdMinor; a negative value means "flag any positive idle", so
	// zero idle cash is never flagged.
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
	// computed exactly on the benchmark's decimal text and truncated toward zero to
	// whole minor units. Zero when the benchmark is zero; saturates at
	// math.MaxInt64 if the exact figure does not fit.
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
	var idle int64
	if in.LiquidMinor > committed {
		idle = in.LiquidMinor - committed
	}

	bench := in.BenchmarkAPRPercent
	if math.IsNaN(bench) || math.IsInf(bench, 0) || bench < 0 {
		bench = 0
	}

	forgoneAnnual := forgoneAnnualMinor(idle, bench)
	forgoneMonthly := forgoneAnnual / 12

	threshold := in.ThresholdMinor
	if threshold == 0 {
		threshold = DefaultThresholdMinor
	}
	flagged := idle >= threshold
	if threshold < 0 {
		flagged = idle > 0
	}

	return Result{
		LiquidMinor:         in.LiquidMinor,
		CommittedMinor:      committed,
		IdleMinor:           idle,
		ForgoneAnnualMinor:  forgoneAnnual,
		ForgoneMonthlyMinor: forgoneMonthly,
		BenchmarkAPRPercent: bench,
		Flag:                flagged && bench > 0,
	}
}

// forgoneAnnualMinor returns idle × percent/100 truncated toward zero. The percent
// is read from its shortest decimal text, so a benchmark typed as 4.1 is exactly
// 4.1 and not the nearest binary fraction, and the product is computed with
// big-integer arithmetic.
func forgoneAnnualMinor(idle int64, percent float64) int64 {
	if idle <= 0 || percent <= 0 {
		return 0
	}
	rate, ok := new(big.Rat).SetString(strconv.FormatFloat(percent, 'f', -1, 64))
	if !ok {
		return 0
	}
	product := new(big.Int).Mul(big.NewInt(idle), rate.Num())
	divisor := new(big.Int).Mul(rate.Denom(), big.NewInt(100))
	quotient := product.Quo(product, divisor)
	if !quotient.IsInt64() {
		return math.MaxInt64
	}
	return quotient.Int64()
}
