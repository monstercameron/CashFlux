// SPDX-License-Identifier: MIT

// Package emergencyfund sizes an emergency fund honestly from the household's
// ESSENTIAL month — fixed commitments plus essential-classified (non-flex)
// spending — rather than from a round-number guess. It produces the derived
// essential-monthly figure and the standard 3- and 6-month fund targets, and a
// drift check the re-suggest flag (GL3 / SMART-G21) uses to notice when the
// derived figure has moved away from the target a goal was last set against.
//
// Pure Go, no platform dependencies; unit-tested on native Go. All amounts are
// integer minor units in a single currency (the caller converts to base first).
package emergencyfund

import "github.com/monstercameron/CashFlux/internal/money"

// Basis is the honest input to sizing, expressed as monthly minor-unit figures
// in one currency. It separates the two components so the UI can explain the
// number ("$1,900 fixed + $1,000 essential spending").
type Basis struct {
	// FixedMonthlyMinor is the sum of fixed recurring commitments (rent, loan
	// payments, insurance, essential subscriptions) normalized to a monthly cost.
	FixedMonthlyMinor int64
	// EssentialSpendMonthlyMinor is the trailing monthly average of essential,
	// non-discretionary categorized spending (groceries, utilities, transport)
	// NOT already counted in FixedMonthlyMinor — i.e. categories classed fixed or
	// non-monthly, excluding flex/discretionary.
	EssentialSpendMonthlyMinor int64
	// Currency is the ISO code both components are denominated in.
	Currency string
}

// EssentialMonthlyMinor is the derived cost of one essential month: the fixed
// commitments plus essential spending. Negative components are clamped to zero
// so a malformed input can never understate the fund.
func (b Basis) EssentialMonthlyMinor() int64 {
	return nonNeg(b.FixedMonthlyMinor) + nonNeg(b.EssentialSpendMonthlyMinor)
}

// Sizing is the derived essential-month figure and the standard fund targets.
type Sizing struct {
	// EssentialMonthly is the cost of one essential month.
	EssentialMonthly money.Money
	// ThreeMonth and SixMonth are the recommended fund targets (3× and 6× the
	// essential month) — the two horizons the one-tap "set as target" offers.
	ThreeMonth money.Money
	SixMonth   money.Money
}

// Level is a supported emergency-fund horizon, in months.
type Level int

const (
	// LevelThree is a starter fund covering three essential months.
	LevelThree Level = 3
	// LevelSix is the fuller fund covering six essential months — the common target.
	LevelSix Level = 6
)

// Valid reports whether l is a supported horizon.
func (l Level) Valid() bool { return l == LevelThree || l == LevelSix }

// Months returns the horizon as a whole number of months.
func (l Level) Months() int { return int(l) }

// Size derives the essential-month figure and the 3-/6-month fund targets from
// the basis. The result is denominated in the basis currency; a zero-value
// basis yields zero-valued money in that currency.
func Size(b Basis) Sizing {
	em := b.EssentialMonthlyMinor()
	return Sizing{
		EssentialMonthly: money.New(em, b.Currency),
		ThreeMonth:       money.New(em*int64(LevelThree), b.Currency),
		SixMonth:         money.New(em*int64(LevelSix), b.Currency),
	}
}

// TargetFor returns the fund target for the given horizon. An unrecognized level
// falls back to the six-month target (the safer default), so callers never get a
// zero-valued surprise from a bad level.
func (s Sizing) TargetFor(level Level) money.Money {
	if level == LevelThree {
		return s.ThreeMonth
	}
	return s.SixMonth
}

// TargetMinor is TargetFor as a bare minor-unit amount, convenient for the smart
// flag and atom which work in minor units.
func (s Sizing) TargetMinor(level Level) int64 { return s.TargetFor(level).Amount }

// TrailingAverageMinor averages a slice of per-month essential-spend totals,
// yielding 0 for an empty slice. It is the honest way to derive
// EssentialSpendMonthlyMinor from a few whole months of history rather than a
// single volatile month.
func TrailingAverageMinor(monthly []int64) int64 {
	if len(monthly) == 0 {
		return 0
	}
	var sum int64
	for _, m := range monthly {
		sum += m
	}
	return sum / int64(len(monthly))
}

// DriftExceeds reports whether the freshly derived essential-month figure
// differs from the basis the target was last set against by more than pct
// percent (in either direction). It returns false when the prior basis is
// non-positive (nothing meaningful to compare against) so the re-suggest flag
// only fires once there is a real, established basis to have drifted from.
func DriftExceeds(priorBasisMinor, derivedMinor int64, pct int) bool {
	if priorBasisMinor <= 0 || pct < 0 {
		return false
	}
	diff := derivedMinor - priorBasisMinor
	if diff < 0 {
		diff = -diff
	}
	return diff*100 > priorBasisMinor*int64(pct)
}

func nonNeg(v int64) int64 {
	if v < 0 {
		return 0
	}
	return v
}
