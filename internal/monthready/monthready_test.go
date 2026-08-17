// SPDX-License-Identifier: MIT

package monthready

import (
	"strings"
	"testing"
)

func TestAReadyMonthIsReady(t *testing.T) {
	r := Assess(Input{ExpectedIncomeMinor: 500_000, IncomeKnown: true, CommittedMinor: 300_000})
	if !r.Ready() {
		t.Errorf("a funded month with room was not ready: %+v", r)
	}
	if r.HeadroomMinor != 200_000 {
		t.Errorf("headroom = %d, want 200000", r.HeadroomMinor)
	}
}

// "Next month: 68/100" cannot be acted on, argued with, or improved. The reader
// learns something is wrong and nothing about what.
func TestItNamesTheGapRatherThanScoringIt(t *testing.T) {
	r := Assess(Input{UnbudgetedCategories: []string{"Dining", "Travel"}})
	if r.Ready() {
		t.Fatal("a month with no income recorded was called ready")
	}
	reasons := strings.Join(r.Reasons(), " | ")
	if !strings.Contains(reasons, "next month's income") {
		t.Errorf("the missing income was not named: %q", reasons)
	}
	if !strings.Contains(reasons, "Dining") || !strings.Contains(reasons, "Travel") {
		t.Errorf("the unbudgeted categories were not named: %q", reasons)
	}
}

// Nobody having said is not the same as zero.
func TestUnknownIncomeIsNotZeroIncome(t *testing.T) {
	unknown := Assess(Input{CommittedMinor: 300_000})
	if unknown.Overcommitted {
		t.Error("a month with no income recorded was declared over-committed")
	}
	if unknown.HeadroomMinor != 0 {
		t.Errorf("headroom = %d, want 0 when income is unknown", unknown.HeadroomMinor)
	}
	zero := Assess(Input{IncomeKnown: true, CommittedMinor: 300_000})
	if !zero.Overcommitted {
		t.Error("a month with a recorded zero income and real commitments was not over-committed")
	}
}

func TestATightMonthIsNotReady(t *testing.T) {
	// $50 of room on $5,000 of income is 1%.
	r := Assess(Input{ExpectedIncomeMinor: 500_000, IncomeKnown: true, CommittedMinor: 495_000})
	if !r.Tight {
		t.Error("a month with 1% headroom was not called tight")
	}
	if r.Ready() {
		t.Error("a tight month was called ready")
	}
	// The inputs being known is not the same as the month working.
	if !r.Trustworthy() {
		t.Error("a tight month with complete inputs was reported as untrustworthy")
	}
}

// These are the charges that blow up a month precisely because they do not
// happen most months.
func TestLargeAnnualChargesAreSurfacedAndSmallOnesAreNot(t *testing.T) {
	r := Assess(Input{
		ExpectedIncomeMinor: 500_000, IncomeKnown: true, CommittedMinor: 100_000,
		AnnualCharges: []Charge{
			{Name: "Car insurance", Minor: -95_000},
			{Name: "Domain renewal", Minor: -1_200},
		},
	})
	if len(r.Large) != 1 || r.Large[0].Name != "Car insurance" {
		t.Errorf("large charges = %+v, want just the premium", r.Large)
	}
	// Magnitudes, so a caller's sign convention is not the package's problem.
	if r.Large[0].Minor != 95_000 {
		t.Errorf("amount = %d, want a positive 95000", r.Large[0].Minor)
	}
}

// Withholding everything until every input is present would make the feature
// useless exactly when it is most needed.
func TestItStillWarnsWhenIncomeIsUnknown(t *testing.T) {
	r := Assess(Input{AnnualCharges: []Charge{{Name: "Car insurance", Minor: -95_000}}})
	if len(r.Large) != 1 {
		t.Error("a household with no income recorded was told nothing about a premium landing")
	}
}
