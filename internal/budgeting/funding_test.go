// SPDX-License-Identifier: MIT

package budgeting

import "testing"

// The reported case: a plan fully assigned against expected income, with a third
// of that income not yet arrived.
func TestFundingReadSeparatesAssignedFromFunded(t *testing.T) {
	f := FundingRead{Expected: 1070916, Received: 696100, Assigned: 1070916}

	if f.FullyFunded() {
		t.Error("FullyFunded = true while $3,748.16 of the plan has no money behind it")
	}
	if got := f.Unfunded(); got != 374816 {
		t.Errorf("Unfunded = %d, want 374816", got)
	}
	// The shortfall is a statement about the INCOME, not the assignments — here
	// they happen to match, because the plan was assigned to the full expectation.
	if got := f.Shortfall(); got != 374816 {
		t.Errorf("Shortfall = %d, want 374816", got)
	}
}

// Unfunded and Shortfall answer different questions and must not be the same
// number in general: a conservative plan can be fully funded while the income
// still came in short.
func TestFundingReadUnfundedAndShortfallAreDifferentQuestions(t *testing.T) {
	f := FundingRead{Expected: 500000, Received: 300000, Assigned: 250000}
	if !f.FullyFunded() {
		t.Error("FullyFunded = false, but only $2,500 was assigned against $3,000 received")
	}
	if got := f.Unfunded(); got != 0 {
		t.Errorf("Unfunded = %d, want 0", got)
	}
	if got := f.Shortfall(); got != 200000 {
		t.Errorf("Shortfall = %d, want 200000 — the income still came in low", got)
	}
}

func TestFundingReadMeaningful(t *testing.T) {
	if (FundingRead{}).Meaningful() {
		t.Error("an empty read claims to say something")
	}
	if !(FundingRead{Received: 100}).Meaningful() {
		t.Error("money received should be enough to report on")
	}
}

func TestFundingReadReceivedPct(t *testing.T) {
	// Received sits against the larger of expectation and plan, matching the bar.
	f := FundingRead{Expected: 100000, Received: 50000, Assigned: 100000}
	if got := f.ReceivedPct(); got != 50 {
		t.Errorf("ReceivedPct = %d, want 50", got)
	}
	// Over-assigned: the scale grows, so the marker moves left rather than
	// pretending the arrived money covers more of a bigger plan.
	over := FundingRead{Expected: 100000, Received: 50000, Assigned: 200000}
	if got := over.ReceivedPct(); got != 25 {
		t.Errorf("ReceivedPct (over-assigned) = %d, want 25", got)
	}
	if got := (FundingRead{}).ReceivedPct(); got != 0 {
		t.Errorf("ReceivedPct with no data = %d, want 0", got)
	}
}

// The one-click reconcile hands a percentage to the bulk adjuster, so it has to
// be a number that adjuster will accept and that a person would recognise.
func TestFundingReadReduceToFitPct(t *testing.T) {
	f := FundingRead{Expected: 1070916, Received: 696100, Assigned: 1070916}
	got := f.ReduceToFitPct()
	if got >= 0 {
		t.Fatalf("ReduceToFitPct = %v, want a reduction", got)
	}
	// Applying it must land on (or just above) the received amount.
	after := AdjustedLimit(f.Assigned, got)
	if after > f.Received+f.Assigned/1000 || after < f.Received-f.Assigned/1000 {
		t.Errorf("scaling by %v gives %d, want about %d (the money received)", got, after, f.Received)
	}
	if !ValidAdjustPct(got) {
		t.Errorf("ReduceToFitPct = %v, which the bulk adjuster would reject", got)
	}
}

func TestFundingReadReduceToFitPctDeclinesWhenItCannotHelp(t *testing.T) {
	// Already funded: nothing to reduce.
	if got := (FundingRead{Expected: 100, Received: 100, Assigned: 100}).ReduceToFitPct(); got != 0 {
		t.Errorf("a funded plan = %v, want 0", got)
	}
	// Nothing assigned: nothing to scale.
	if got := (FundingRead{Expected: 100, Received: 50, Assigned: 0}).ReduceToFitPct(); got != 0 {
		t.Errorf("an empty plan = %v, want 0", got)
	}
	// A cut deeper than the adjuster can express must NOT be offered as a
	// one-click fix that would quietly do something else.
	if got := (FundingRead{Expected: 100000, Received: 1, Assigned: 100000}).ReduceToFitPct(); got != 0 {
		t.Errorf("a >90%% cut = %v, want 0 (out of the adjuster's range)", got)
	}
}
