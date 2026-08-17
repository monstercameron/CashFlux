// SPDX-License-Identifier: MIT

// Package monthready says whether next month is set up, and what is missing when
// it is not (EC-11).
//
// # Not a score
//
// The ticket asks for a "readiness score", and a score is the one thing this
// must not produce. "Next month: 68/100" cannot be acted on, argued with, or
// improved — the reader learns that something is wrong and nothing about what.
// So this reports the SPECIFIC gaps, and leans on internal/trust for the verdict
// so the app has one vocabulary for "how much can I rely on this" rather than a
// second scale that has to be learned.
//
// # What "ready" means here
//
// Three questions, each of which a household can actually answer:
//
//   - Is income expected at all? A month with no income on the books is either a
//     gap somebody should know about or a plan nobody entered.
//   - Is anything large and annual landing? These are the charges that blow up a
//     month precisely because they do not happen most months.
//   - Are the committed amounts covered by the expected income? Not "are the
//     budgets set" — a budget is an intention, and the question is whether the
//     money arrives.
package monthready

import (
	"github.com/monstercameron/CashFlux/internal/trust"
)

// LargeAnnualMinor is the size at which a once-a-year charge is worth warning
// about ahead of time.
//
// Two hundred currency units. Below that an annual charge lands without
// reshaping a month, and warning about every yearly $30 domain renewal would
// bury the insurance premium that actually matters.
const LargeAnnualMinor = 200_00

// TightMarginPct is how little headroom counts as tight.
//
// Ten percent. A month whose committed spending is within a tenth of its
// expected income is not a crisis, but it is a month with no room for the
// ordinary surprise — which is exactly the thing worth saying in advance rather
// than afterwards.
const TightMarginPct = 10.0

// Input is what is known about the month ahead. Callers derive it from the
// recurring schedule, the goal plans and the income history; this package makes
// no claim about how those are found.
type Input struct {
	// ExpectedIncomeMinor is what is expected to arrive. Zero with
	// IncomeKnown=false means nobody has said — which is not the same as zero
	// income, and is the distinction the whole assessment turns on.
	ExpectedIncomeMinor int64
	IncomeKnown         bool
	// CommittedMinor is what is already promised: recurring bills, goal
	// contributions, minimum payments.
	CommittedMinor int64
	// AnnualCharges are the large once-a-year items landing in the month.
	AnnualCharges []Charge
	// UnbudgetedCategories are categories that saw spending recently and have no
	// budget for the month ahead. Named, not counted, so the gap is actionable.
	UnbudgetedCategories []string
}

// Charge is one large irregular item landing in the month.
type Charge struct {
	Name  string
	Minor int64
}

// Readiness is what is known and what is missing.
type Readiness struct {
	// Assessment is the shared trust verdict, so "next month is not ready" and
	// "this payoff date is unreliable" speak the same language.
	trust.Assessment
	// Large is the annual charges worth knowing about in advance.
	Large []Charge
	// HeadroomMinor is expected income minus everything committed. Negative means
	// the month is already over-committed before any discretionary spending.
	HeadroomMinor int64
	// Tight says the headroom is real but thin.
	Tight bool
	// Overcommitted says the committed amounts exceed the expected income.
	Overcommitted bool
}

// Assess reads the month ahead.
//
// It reports on what IS known rather than refusing wholesale: a household with
// no income recorded still benefits from knowing an insurance premium lands next
// month, and withholding that until every input is present would make the
// feature useless exactly when it is most needed.
func Assess(in Input) Readiness {
	var r Readiness
	for _, c := range in.AnnualCharges {
		if abs64(c.Minor) >= LargeAnnualMinor {
			r.Large = append(r.Large, Charge{Name: c.Name, Minor: abs64(c.Minor)})
		}
	}

	inputs := []trust.Input{{
		Name:     "next month's income",
		Required: true,
		Missing:  !in.IncomeKnown,
	}}
	if in.IncomeKnown {
		r.HeadroomMinor = in.ExpectedIncomeMinor - in.CommittedMinor
		r.Overcommitted = r.HeadroomMinor < 0
		if !r.Overcommitted && in.ExpectedIncomeMinor > 0 {
			r.Tight = float64(r.HeadroomMinor)/float64(in.ExpectedIncomeMinor)*100 < TightMarginPct
		}
	}
	// An unbudgeted category is not a missing FACT — the money is known, nobody
	// has decided what to do with it — so it qualifies the month rather than
	// invalidating it.
	for _, name := range in.UnbudgetedCategories {
		inputs = append(inputs, trust.Input{Name: "a budget for " + name, Missing: true})
	}
	r.Assessment = trust.Assess(inputs)
	return r
}

// Ready reports whether the month needs nothing from the household. It is
// deliberately strict: a tight or over-committed month is not ready even when
// every input is present, because the inputs being known is not the same as the
// month working.
func (r Readiness) Ready() bool {
	return r.Trustworthy() && !r.Tight && !r.Overcommitted
}

func abs64(v int64) int64 {
	if v < 0 {
		return -v
	}
	return v
}
