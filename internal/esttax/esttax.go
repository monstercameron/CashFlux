// SPDX-License-Identifier: MIT

// Package esttax estimates quarterly tax payments for someone with untaxed
// income (FP-T1e, part 3).
//
// The problem it addresses is not arithmetic, it is timing. Someone whose income
// arrives without withholding owes tax four times a year, and the penalty for
// under-paying accrues per quarter — so being right in April does not undo being
// short in June. The app already knows the income and the deductible spending;
// what it never did was say "this is what you should send, and when".
//
// # Why the safe harbor is the headline and not the estimate
//
// A projection of this year's tax is a guess: income has not finished happening,
// and every guess about it is wrong by some amount. The SAFE HARBOR is not a
// guess — it is a rule about LAST year's tax, a number that is already final, and
// paying it removes the penalty regardless of how this year turns out. Leading
// with the estimate would offer precision the data cannot support in place of a
// guarantee it can.
package esttax

import (
	"math"
	"time"
)

// SafeHarborPct is the share of last year's tax that must be paid this year to
// avoid an underpayment penalty.
//
// The higher figure applies above an income threshold. Both are conventions of a
// particular tax system that change, which is why they are named constants shown
// as stated assumptions rather than magic numbers folded into a formula.
const (
	SafeHarborPct     = 100.0
	SafeHarborHighPct = 110.0
	// HighIncomeThresholdMinor is the prior-year income above which the higher
	// safe-harbor share applies ($150,000).
	HighIncomeThresholdMinor = 15_000_000
	// CurrentYearPct is the alternative harbor: this year's actual tax.
	CurrentYearPct = 90.0
)

// Inputs is what an estimate needs.
type Inputs struct {
	// NetIncomeMinor is income less deductible expense for the year SO FAR.
	NetIncomeMinor int64
	// EffectiveRatePct is the household's own blended rate — income tax plus
	// self-employment tax. Stated by the user, not derived: this app does not know
	// the filing status, the state, or the other income that sets it, and a rate
	// invented from a bracket table would be wrong in a way that looks computed.
	EffectiveRatePct float64
	// PriorYearTaxMinor is what was owed last year. Zero means unknown, which
	// removes the safe harbor rather than treating last year's tax as nothing.
	PriorYearTaxMinor int64
	// PriorYearIncomeMinor decides which safe-harbor share applies. Zero means
	// unknown, and the lower share is assumed — the conservative direction here is
	// the one that does not tell someone to pay less than they may owe, so unknown
	// income keeps the estimate honest by leaving the higher tier unclaimed.
	PriorYearIncomeMinor int64
	// PaidToDateMinor is what has already been sent this year.
	PaidToDateMinor int64
	// Now is the date the estimate is made from.
	Now time.Time
}

// Estimate is the answer.
type Estimate struct {
	// ProjectedTaxMinor is this year's tax at the stated rate on income so far,
	// scaled to a full year.
	ProjectedTaxMinor int64
	// SafeHarborMinor is the total that removes the penalty, and Known says
	// whether one could be computed at all.
	SafeHarborMinor int64
	SafeHarborKnown bool
	SafeHarborPct   float64
	// TargetMinor is what to aim at for the year: the LOWER of the projection and
	// the safe harbor when both exist, because either satisfies the rule and
	// asking for the larger one is asking for an interest-free loan.
	TargetMinor int64
	// DueNowMinor is what should have been paid by the current quarter, net of
	// what has been paid. Negative means ahead, and is reported as ahead rather
	// than clamped to zero — being ahead is information.
	DueNowMinor int64
	// Quarter is the quarter Now falls in (1–4), and QuarterDue is its deadline.
	Quarter    int
	QuarterDue time.Time
}

// Compute produces the estimate.
//
// Reports ok=false when the rate is unusable or income so far is not positive.
// Zero income is not "you owe nothing" — it is a year that has not produced a
// figure yet, and a confident $0 due would be read as a green light.
func Compute(in Inputs) (Estimate, bool) {
	if in.EffectiveRatePct <= 0 || in.EffectiveRatePct >= 100 || in.Now.IsZero() {
		return Estimate{}, false
	}
	if in.NetIncomeMinor <= 0 {
		return Estimate{}, false
	}
	var e Estimate
	e.Quarter, e.QuarterDue = QuarterOf(in.Now)

	// Income so far is scaled to the year by elapsed days rather than by quarter,
	// because a quarter boundary crossed yesterday would otherwise jump the
	// projection by a third overnight on no new information.
	elapsed := float64(in.Now.YearDay())
	if elapsed < 1 {
		elapsed = 1
	}
	yearLen := 365.0
	if isLeap(in.Now.Year()) {
		yearLen = 366.0
	}
	projectedIncome := float64(in.NetIncomeMinor) * yearLen / elapsed
	e.ProjectedTaxMinor = int64(math.Round(projectedIncome * in.EffectiveRatePct / 100))

	if in.PriorYearTaxMinor > 0 {
		pct := SafeHarborPct
		if in.PriorYearIncomeMinor > HighIncomeThresholdMinor {
			pct = SafeHarborHighPct
		}
		e.SafeHarborKnown = true
		e.SafeHarborPct = pct
		e.SafeHarborMinor = int64(math.Round(float64(in.PriorYearTaxMinor) * pct / 100))
	}

	e.TargetMinor = e.ProjectedTaxMinor
	if e.SafeHarborKnown && e.SafeHarborMinor < e.TargetMinor {
		e.TargetMinor = e.SafeHarborMinor
	}

	// The year's target is paid in four equal instalments, so by the end of
	// quarter N, N quarters' worth should have gone.
	shouldHavePaid := e.TargetMinor * int64(e.Quarter) / 4
	e.DueNowMinor = shouldHavePaid - in.PaidToDateMinor
	return e, true
}

// QuarterOf returns the estimated-tax quarter a date falls in and that quarter's
// payment deadline.
//
// The quarters are NOT calendar quarters, and this is the detail that catches
// people: the periods run Jan–Mar, Apr–May, Jun–Aug, Sep–Dec, so the second is
// two months long and the third and fourth are uneven. Treating them as calendar
// quarters puts two of the four deadlines in the wrong place.
func QuarterOf(now time.Time) (int, time.Time) {
	y := now.Year()
	switch m := now.Month(); {
	case m <= time.March:
		return 1, date(y, time.April, 15)
	case m <= time.May:
		return 2, date(y, time.June, 15)
	case m <= time.August:
		return 3, date(y, time.September, 15)
	default:
		// The fourth payment is due in JANUARY of the following year.
		return 4, date(y+1, time.January, 15)
	}
}

// date builds a UTC midnight date.
func date(y int, m time.Month, d int) time.Time {
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

// isLeap reports whether a year has 366 days.
func isLeap(y int) bool { return (y%4 == 0 && y%100 != 0) || y%400 == 0 }
