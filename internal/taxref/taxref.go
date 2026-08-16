// SPDX-License-Identifier: MIT

// Package taxref holds dated US contribution limits and thresholds, so the
// assistant can quote one instead of recalling it (PS9).
//
// A model asked "how much can I put in my 401(k)?" will answer confidently and be
// a year out, because its recollection of these numbers is frozen at training time
// and they change annually. That failure is uniquely bad here: it is specific,
// plausible, actionable and wrong, which is the exact shape of an error somebody
// acts on without checking.
//
// So the numbers live in a table with a YEAR on them, and everything that reads
// them reports that year alongside the figure. A stale table is then visible
// ("these are the 2026 limits") rather than invisible; a model's stale recollection
// is not. When a year is not in the table the answer is that we do not have it,
// which is the one response that cannot mislead.
//
// This is reference data, not advice. Nothing here interprets a household's
// situation, and the assistant is instructed to say the same.
//
// Pure: no syscall/js. Table-tested on native Go.
package taxref

import (
	"fmt"
	"sort"
)

// Limits is one tax year's figures, in whole dollars.
type Limits struct {
	// Year is the tax year these apply to. It is reported with every figure.
	Year int

	// Retirement contribution limits.
	Contrib401k         int
	Contrib401kCatchUp  int
	ContribIRA          int
	ContribIRACatchUp   int
	CatchUpAge          int
	ContribHSASelf      int
	ContribHSAFamily    int
	ContribHSACatchUp   int
	ContribHSACatchUpAt int

	// StandardDeductionSingle and StandardDeductionJoint are the standard
	// deductions for the two most common filing statuses.
	StandardDeductionSingle int
	StandardDeductionJoint  int

	// SocialSecurityWageBase is the earnings ceiling for Social Security tax.
	SocialSecurityWageBase int
	// FullRetirementAge is the age at which unreduced Social Security begins, for
	// somebody born in 1960 or later.
	FullRetirementAge int
	// EarliestClaimAge and LatestCreditAge bound when benefits can be claimed and
	// when delayed-retirement credits stop accruing.
	EarliestClaimAge int
	LatestCreditAge  int
}

// years is the table. Every figure is entered against the year it applies to; a
// year that is not here is simply not known, which is the honest answer to give.
//
// These are the published US federal figures. When they are updated, the OLD year
// stays: somebody doing last year's taxes needs last year's numbers, and quietly
// overwriting them would make the app wrong for exactly the person most likely to
// be looking.
var years = map[int]Limits{
	2025: {
		Year:                    2025,
		Contrib401k:             23500,
		Contrib401kCatchUp:      7500,
		ContribIRA:              7000,
		ContribIRACatchUp:       1000,
		CatchUpAge:              50,
		ContribHSASelf:          4300,
		ContribHSAFamily:        8550,
		ContribHSACatchUp:       1000,
		ContribHSACatchUpAt:     55,
		StandardDeductionSingle: 15000,
		StandardDeductionJoint:  30000,
		SocialSecurityWageBase:  176100,
		FullRetirementAge:       67,
		EarliestClaimAge:        62,
		LatestCreditAge:         70,
	},
	2026: {
		Year:                    2026,
		Contrib401k:             24500,
		Contrib401kCatchUp:      8000,
		ContribIRA:              7500,
		ContribIRACatchUp:       1100,
		CatchUpAge:              50,
		ContribHSASelf:          4400,
		ContribHSAFamily:        8750,
		ContribHSACatchUp:       1000,
		ContribHSACatchUpAt:     55,
		StandardDeductionSingle: 15750,
		StandardDeductionJoint:  31500,
		SocialSecurityWageBase:  184500,
		FullRetirementAge:       67,
		EarliestClaimAge:        62,
		LatestCreditAge:         70,
	},
}

// For returns a tax year's figures. ok is false for a year that is not in the
// table — the caller must then say it does not know, rather than reaching for the
// nearest year, which would be wrong by exactly the amount that matters.
func For(year int) (Limits, bool) {
	l, ok := years[year]
	return l, ok
}

// Years returns every year the table covers, oldest first, so a caller can say
// what it does have.
func Years() []int {
	out := make([]int, 0, len(years))
	for y := range years {
		out = append(out, y)
	}
	sort.Ints(out)
	return out
}

// Latest returns the most recent year in the table.
func Latest() Limits {
	all := Years()
	if len(all) == 0 {
		return Limits{}
	}
	return years[all[len(all)-1]]
}

// Vintage is the sentence every quoted figure carries. Showing the year is the
// whole mechanism: it turns a stale number from a silent error into a visible one.
func (l Limits) Vintage() string {
	return fmt.Sprintf("%d figures", l.Year)
}

// Summary renders the year's headline figures as plain lines, for the assistant to
// quote. It ends by naming the year again and disclaiming advice — the numbers are
// reference data, and the difference between quoting a limit and telling somebody
// what to do with it is the whole reason this can exist at all.
func (l Limits) Summary() string {
	if l.Year == 0 {
		return "No figures available."
	}
	return fmt.Sprintf(`US federal figures for %d:
- 401(k) employee contribution: $%s (plus $%s catch-up from age %d)
- IRA contribution: $%s (plus $%s catch-up from age %d)
- HSA contribution: $%s self-only, $%s family (plus $%s catch-up from age %d)
- Standard deduction: $%s single, $%s married filing jointly
- Social Security wage base: $%s
- Social Security: earliest claim %d, full retirement age %d, credits stop at %d

These are the published %d figures, quoted as reference. They are not advice, and they change annually.`,
		l.Year,
		commas(l.Contrib401k), commas(l.Contrib401kCatchUp), l.CatchUpAge,
		commas(l.ContribIRA), commas(l.ContribIRACatchUp), l.CatchUpAge,
		commas(l.ContribHSASelf), commas(l.ContribHSAFamily), commas(l.ContribHSACatchUp), l.ContribHSACatchUpAt,
		commas(l.StandardDeductionSingle), commas(l.StandardDeductionJoint),
		commas(l.SocialSecurityWageBase),
		l.EarliestClaimAge, l.FullRetirementAge, l.LatestCreditAge,
		l.Year)
}

// commas renders a whole-dollar figure with thousands separators.
func commas(n int) string {
	s := fmt.Sprintf("%d", n)
	if len(s) <= 3 {
		return s
	}
	var out []byte
	pre := len(s) % 3
	if pre > 0 {
		out = append(out, s[:pre]...)
	}
	for i := pre; i < len(s); i += 3 {
		if len(out) > 0 {
			out = append(out, ',')
		}
		out = append(out, s[i:i+3]...)
	}
	return string(out)
}
