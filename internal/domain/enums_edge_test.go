package domain

import "testing"

// TestEnumInvalidRemaining covers the default (invalid) branch of the Valid
// methods not already exercised by TestEnumInvalid.
func TestEnumInvalidRemaining(t *testing.T) {
	if Scope("nope").Valid() {
		t.Error("invalid Scope should be invalid")
	}
	if Period("nope").Valid() {
		t.Error("invalid Period should be invalid")
	}
	if TaskStatus("nope").Valid() {
		t.Error("invalid TaskStatus should be invalid")
	}
	if RelatedType("nope").Valid() {
		t.Error("invalid RelatedType should be invalid")
	}
	if TaskSource("nope").Valid() {
		t.Error("invalid TaskSource should be invalid")
	}
}

// TestPeriodLabel covers Period.Label for every period plus the default.
func TestPeriodLabel(t *testing.T) {
	cases := map[Period]string{
		PeriodWeekly:    "Weekly",
		PeriodMonthly:   "Monthly",
		PeriodQuarterly: "Quarterly",
		Period("nope"):  "Monthly", // an unknown period falls back to Monthly
	}
	for p, want := range cases {
		if got := p.Label(); got != want {
			t.Errorf("Period(%q).Label() = %q, want %q", string(p), got, want)
		}
	}
}
