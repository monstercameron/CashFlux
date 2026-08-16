// SPDX-License-Identifier: MIT

package taxref

import (
	"fmt"
	"strings"
	"testing"
)

func TestAYearWeDoNotHaveIsRefusedNotApproximated(t *testing.T) {
	// Reaching for the nearest year would be wrong by exactly the amount that
	// matters, and would look right.
	if _, ok := For(1999); ok {
		t.Fatal("an unlisted year returned figures")
	}
	if _, ok := For(2099); ok {
		t.Fatal("a future year returned figures")
	}
}

func TestEveryYearInTheTableIsComplete(t *testing.T) {
	// A half-filled year quotes a $0 limit, which is a specific, plausible, wrong
	// number — the worst kind here.
	for _, y := range Years() {
		l, ok := For(y)
		if !ok {
			t.Fatalf("Years() lists %d but For cannot resolve it", y)
		}
		for name, v := range map[string]int{
			"401k": l.Contrib401k, "401k catch-up": l.Contrib401kCatchUp,
			"IRA": l.ContribIRA, "IRA catch-up": l.ContribIRACatchUp,
			"HSA self": l.ContribHSASelf, "HSA family": l.ContribHSAFamily,
			"standard deduction (single)": l.StandardDeductionSingle,
			"standard deduction (joint)":  l.StandardDeductionJoint,
			"SS wage base":                l.SocialSecurityWageBase,
			"catch-up age":                l.CatchUpAge,
			"full retirement age":         l.FullRetirementAge,
		} {
			if v <= 0 {
				t.Errorf("%d has no %s", y, name)
			}
		}
		if l.Year != y {
			t.Errorf("the %d entry reports itself as %d", y, l.Year)
		}
	}
}

func TestFiguresRiseYearOnYearOrStay(t *testing.T) {
	// A contribution limit that FELL between years is almost certainly a typo, and
	// a typo here is a number somebody acts on.
	all := Years()
	for i := 1; i < len(all); i++ {
		prev, _ := For(all[i-1])
		cur, _ := For(all[i])
		if cur.Contrib401k < prev.Contrib401k {
			t.Errorf("the 401(k) limit fell from %d (%d) to %d (%d)", prev.Contrib401k, prev.Year, cur.Contrib401k, cur.Year)
		}
		if cur.ContribIRA < prev.ContribIRA {
			t.Errorf("the IRA limit fell from %d to %d", prev.ContribIRA, cur.ContribIRA)
		}
		if cur.SocialSecurityWageBase < prev.SocialSecurityWageBase {
			t.Errorf("the SS wage base fell from %d to %d", prev.SocialSecurityWageBase, cur.SocialSecurityWageBase)
		}
	}
}

func TestOldYearsAreKeptWhenNewOnesArrive(t *testing.T) {
	// Somebody doing last year's taxes needs last year's numbers; overwriting them
	// makes the app wrong for exactly the person most likely to be looking.
	if len(Years()) < 2 {
		t.Fatal("only one year is in the table, so last year's figures are already gone")
	}
}

func TestEveryQuotedFigureCarriesItsYear(t *testing.T) {
	// This is the whole mechanism: a stale table is visible, a model's stale
	// recollection is not.
	l := Latest()
	if !strings.Contains(l.Vintage(), "figures") || !strings.Contains(l.Summary(), "not advice") {
		t.Fatalf("vintage = %q", l.Vintage())
	}
	summary := l.Summary()
	year := fmt.Sprintf("%d", l.Year)
	if strings.Count(summary, year) < 2 {
		t.Fatalf("the summary names its year fewer than twice:\n%s", summary)
	}
}

func TestTheSummaryReadsAsNumbersNotAdvice(t *testing.T) {
	summary := Latest().Summary()
	for _, forbidden := range []string{"you should", "we recommend", "the best", "you ought"} {
		if strings.Contains(strings.ToLower(summary), forbidden) {
			t.Errorf("the summary gives advice: it contains %q", forbidden)
		}
	}
}

func TestAnEmptyLimitsSaysSoRatherThanQuotingZeroes(t *testing.T) {
	if got := (Limits{}).Summary(); got != "No figures available." {
		t.Fatalf("an empty Limits summarised as %q", got)
	}
}

func TestThousandsSeparatorsAreCorrect(t *testing.T) {
	for _, tc := range []struct {
		n    int
		want string
	}{
		{0, "0"}, {999, "999"}, {1000, "1,000"}, {24500, "24,500"}, {184500, "184,500"},
	} {
		if got := commas(tc.n); got != tc.want {
			t.Errorf("commas(%d) = %q, want %q", tc.n, got, tc.want)
		}
	}
}
