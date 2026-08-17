// SPDX-License-Identifier: MIT

// Package retirement projects a long-horizon balance forward to a retirement
// date and then back down through the years it has to cover (FP-T1a, FP-T1b).
//
// `TypeRetirement` was only an account type — a label with no arithmetic behind
// it. The two questions people actually have about it are "how much will I have"
// and "how long will it last", and they are one arc: accumulation and
// decumulation share a balance, a return assumption and an inflation assumption,
// and answering them separately is how the two halves come to disagree.
//
// Four commitments run through everything here:
//
//   - REAL dollars are the headline. A nominal projection of $2.1M in 2056 is a
//     technically-correct number that means nothing to a person, because they
//     cannot price 2056 groceries. Every result carries both, and the real figure
//     is the one worth showing first.
//   - Annual granularity. Monthly compounding on a thirty-year horizon adds
//     precision that the inputs — a guessed return, a guessed inflation rate, a
//     contribution that will certainly change — cannot support. False precision
//     on a projection invites people to act on digits that are noise.
//   - It reports what it ASSUMED. A projection is arithmetic over guesses, and a
//     figure whose assumptions are invisible is a figure that gets believed.
//   - It refuses rather than guessing. No return assumption, no horizon, a
//     negative rate that makes the maths meaningless — each returns not-known
//     rather than a number.
//
// Pure Go, no platform dependencies, table-tested.
package retirement

import "math"

// MaxYears bounds every projection. A hundred and twenty years is past any human
// horizon, and the bound is what stops a zero-withdrawal drawdown from looping
// forever.
const MaxYears = 120

// Assumptions are the guesses a projection rests on. They are one struct so a
// caller cannot supply half of them, and so the result can echo them back.
type Assumptions struct {
	// ReturnPct is the expected nominal annual return, as a percent (7 = 7%).
	ReturnPct float64
	// InflationPct is the expected annual inflation, as a percent.
	InflationPct float64
}

// RealReturnPct is the return net of inflation, via the Fisher relation rather
// than a subtraction.
//
// (1+r)/(1+i)-1, not r-i. At 7% and 3% the difference is 3.88% versus 4.00% —
// small annually and roughly a 4% error in the final balance over thirty years,
// which is a whole year of contributions. The exact form costs one division.
func (a Assumptions) RealReturnPct() float64 {
	return ((1+a.ReturnPct/100)/(1+a.InflationPct/100) - 1) * 100
}

// Valid reports whether the assumptions can support a projection. A return at or
// below -100% is not a market outcome, it is an arithmetic hole.
func (a Assumptions) Valid() bool {
	return a.ReturnPct > -100 && a.InflationPct > -100
}

// Year is one year of a projection.
type Year struct {
	// Index is years from the start (0 is today's balance, before any growth).
	Index int
	// NominalMinor is the balance in that year's own dollars.
	NominalMinor int64
	// RealMinor is the balance in TODAY's dollars — the figure a person can price.
	RealMinor int64
	// ContributedMinor / WithdrawnMinor are that year's flows, nominal.
	ContributedMinor, WithdrawnMinor int64
}

// Projection is an accumulation run.
type Projection struct {
	Years []Year
	// FinalNominalMinor / FinalRealMinor are the balance at the horizon.
	FinalNominalMinor, FinalRealMinor int64
	// ContributedMinor is everything paid in, nominal and undiscounted — the
	// number to set against the final balance to see what the market did versus
	// what the saver did.
	ContributedMinor int64
	// Assumptions echoes what this rests on, so a surface can state it without
	// holding the inputs itself.
	Assumptions Assumptions
	// Known is false when the inputs could not support a projection.
	Known bool
}

// GrowthMinor is the final balance minus everything paid in: what the returns
// contributed, as opposed to the saver. Nominal, because comparing a real
// balance against nominal contributions would flatter or punish it arbitrarily.
func (p Projection) GrowthMinor() int64 {
	return p.FinalNominalMinor - p.ContributedMinor - p.startMinor()
}

func (p Projection) startMinor() int64 {
	if len(p.Years) == 0 {
		return 0
	}
	return p.Years[0].NominalMinor
}

// Project grows a starting balance for `years`, adding annualContribution at the
// END of each year.
//
// End-of-year, not start: a contribution made through the year has, on average,
// been invested for about half of it, and crediting a full year's growth to
// money that arrived in December is the single most common way a retirement
// projection flatters itself. End-of-year is the conservative convention and the
// one most calculators use.
//
// Returns Known=false for a non-positive horizon or invalid assumptions —
// never a zero balance, which would read as "you will have nothing".
func Project(startMinor, annualContributionMinor int64, years int, a Assumptions) Projection {
	if years <= 0 || years > MaxYears || !a.Valid() {
		return Projection{Assumptions: a}
	}
	p := Projection{Assumptions: a, Known: true, Years: make([]Year, 0, years+1)}
	r := a.ReturnPct / 100
	infl := a.InflationPct / 100

	bal := float64(startMinor)
	p.Years = append(p.Years, Year{Index: 0,
		NominalMinor: startMinor, RealMinor: startMinor})

	for i := 1; i <= years; i++ {
		bal *= 1 + r
		bal += float64(annualContributionMinor)
		p.ContributedMinor += annualContributionMinor
		// Discount by inflation compounded over the same number of years, so the
		// real figure answers "what is this worth in today's money".
		real := bal / math.Pow(1+infl, float64(i))
		p.Years = append(p.Years, Year{
			Index: i, NominalMinor: int64(math.Round(bal)),
			RealMinor:        int64(math.Round(real)),
			ContributedMinor: annualContributionMinor,
		})
	}
	last := p.Years[len(p.Years)-1]
	p.FinalNominalMinor, p.FinalRealMinor = last.NominalMinor, last.RealMinor
	return p
}

// Drawdown is a decumulation run: how long a nest egg lasts.
type Drawdown struct {
	Years []Year
	// LastsYears is how many whole years the balance covers.
	LastsYears int
	// Depleted reports whether it ran out inside the horizon. When false, the
	// balance survived the whole horizon — which is the GOOD outcome and must not
	// be confused with "we did not check".
	Depleted bool
	// EndingNominalMinor / EndingRealMinor are what is left at the horizon (zero
	// when depleted).
	EndingNominalMinor, EndingRealMinor int64
	Assumptions                         Assumptions
	// Known is false when the inputs could not support a run.
	Known bool
}

// Drawdown spends a balance down at an annual withdrawal that grows with
// inflation, for at most `horizonYears`.
//
// The withdrawal is inflation-INDEXED: someone withdrawing $60,000 today needs
// more than $60,000 in ten years to buy the same life. A flat nominal withdrawal
// would report a nest egg lasting far longer than it does, which is the failure
// mode of every over-simple retirement calculator.
//
// Withdrawals come out at the START of the year, before growth. A year's spending
// is not available to compound through the year it is spent, and assuming
// otherwise is the mirror of the end-of-year contribution convention above.
func RunDrawdown(startMinor, firstYearWithdrawalMinor int64, horizonYears int, a Assumptions) Drawdown {
	if horizonYears <= 0 || horizonYears > MaxYears || !a.Valid() || firstYearWithdrawalMinor <= 0 {
		return Drawdown{Assumptions: a}
	}
	d := Drawdown{Assumptions: a, Known: true, Years: make([]Year, 0, horizonYears+1)}
	r := a.ReturnPct / 100
	infl := a.InflationPct / 100

	bal := float64(startMinor)
	d.Years = append(d.Years, Year{Index: 0, NominalMinor: startMinor, RealMinor: startMinor})

	for i := 1; i <= horizonYears; i++ {
		want := float64(firstYearWithdrawalMinor) * math.Pow(1+infl, float64(i-1))
		took := want
		if took > bal {
			// The final year takes what is left, not what was wanted — reporting a
			// withdrawal the balance could not fund would overstate the last year.
			took = bal
		}
		bal -= took
		if bal <= 0 {
			bal = 0
			d.Years = append(d.Years, Year{Index: i, NominalMinor: 0, RealMinor: 0,
				WithdrawnMinor: int64(math.Round(took))})
			d.Depleted, d.LastsYears = true, i-1
			return d
		}
		bal *= 1 + r
		real := bal / math.Pow(1+infl, float64(i))
		d.Years = append(d.Years, Year{Index: i,
			NominalMinor: int64(math.Round(bal)), RealMinor: int64(math.Round(real)),
			WithdrawnMinor: int64(math.Round(took))})
	}
	last := d.Years[len(d.Years)-1]
	d.LastsYears = horizonYears
	d.EndingNominalMinor, d.EndingRealMinor = last.NominalMinor, last.RealMinor
	return d
}

// DefaultSWRPct is the withdrawal rate the FIRE number defaults to.
//
// Four percent, from the Trinity study, which is a rule of thumb about a
// specific historical period and a specific portfolio — not a law. It is exposed
// as a constant and as a parameter precisely so a surface can say where the
// number came from rather than presenting it as arithmetic.
const DefaultSWRPct = 4.0

// FIRENumber is the nest egg that supports annualExpenses at the given safe
// withdrawal rate: expenses ÷ rate.
//
// Returns ok=false for a non-positive rate or expenses. A zero rate is a
// division by zero dressed as "never withdraw"; a zero expense figure means
// nobody has said what the life costs, and answering "you need nothing" would be
// a confident absurdity.
func FIRENumber(annualExpensesMinor int64, swrPct float64) (int64, bool) {
	if annualExpensesMinor <= 0 || swrPct <= 0 {
		return 0, false
	}
	return int64(math.Round(float64(annualExpensesMinor) / (swrPct / 100))), true
}

// YearsToFI solves how many whole years of saving reach a target, given a
// starting balance, an annual contribution and a REAL return.
//
// Real, not nominal, because the target is expressed in today's money — mixing a
// nominal growth path with a real target is the single easiest way to be
// cheerfully years wrong.
//
// ok=false when the target is unreachable on these inputs: no contribution and
// no growth, or growth that never closes the gap inside MaxYears. Reporting a
// large number instead would present "never" as "eventually".
func YearsToFI(startMinor, annualContributionMinor, targetMinor int64, realReturnPct float64) (int, bool) {
	if targetMinor <= 0 {
		return 0, false
	}
	if startMinor >= targetMinor {
		return 0, true
	}
	if annualContributionMinor <= 0 && realReturnPct <= 0 {
		return 0, false
	}
	r := realReturnPct / 100
	bal := float64(startMinor)
	for i := 1; i <= MaxYears; i++ {
		bal *= 1 + r
		bal += float64(annualContributionMinor)
		if bal >= float64(targetMinor) {
			return i, true
		}
	}
	return 0, false
}
