// SPDX-License-Identifier: MIT

package monthlyreview

import (
	"strings"
	"testing"
	"time"
)

var reviewNow = time.Date(2026, time.August, 16, 12, 0, 0, 0, time.UTC)

func full() Availability {
	return Availability{HasFindings: true, BudgetsNeedingAttention: 2, GoalsNeedingAttention: 1}
}

func TestAStepWithNothingInItIsSkippedNotShownEmpty(t *testing.T) {
	// "No budgets need attention" is worth one line in the close, not a screen of
	// its own with a Next button.
	got := VisibleSteps(Availability{HasFindings: true})
	want := []Step{StepRecap, StepFindings, StepClose}
	if len(got) != len(want) {
		t.Fatalf("steps = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("steps = %v, want %v", got, want)
		}
	}
}

func TestTheRecapAndTheCloseAreAlwaysThere(t *testing.T) {
	// The recap is the orientation everything else depends on; the close is what
	// makes the review feel finished rather than abandoned.
	got := VisibleSteps(Availability{})
	if len(got) != 2 || got[0] != StepRecap || got[1] != StepClose {
		t.Fatalf("steps = %v", got)
	}
}

func TestAMonthWithNothingToReviewIsNotOffered(t *testing.T) {
	// Opening a ten-minute ritual to report that there is nothing to do wastes
	// the ten minutes it asked for.
	if Worthwhile(Availability{}) {
		t.Fatal("an empty month was called worthwhile")
	}
	if ShouldOffer(Progress{}, Availability{}, reviewNow) {
		t.Fatal("an empty month was offered")
	}
}

func TestADismissalIsRespectedForTheRestOfTheMonth(t *testing.T) {
	// Offering it again the next day is exactly the nagging the app promises not
	// to do.
	p := Dismiss(Progress{}, reviewNow)
	if ShouldOffer(p, full(), reviewNow.AddDate(0, 0, 1)) {
		t.Fatal("a dismissed review came back the next day")
	}
	// A new month starts clean.
	if !ShouldOffer(p, full(), reviewNow.AddDate(0, 1, 0)) {
		t.Fatal("a dismissal in August silenced September too")
	}
}

func TestAskingForItBackOutranksTheEarlierNotNow(t *testing.T) {
	p := Reopen(Dismiss(Progress{}, reviewNow))
	if !ShouldOffer(p, full(), reviewNow) {
		t.Fatal("reopening did not bring the review back")
	}
}

func TestProgressFromAnotherMonthIsDiscardedNotResumedInto(t *testing.T) {
	// A half-finished July review has nothing to say about August, and resuming
	// into it would show last month's figures under this month's heading.
	july := Progress{Month: "2026-07", StepIndex: 3, Skipped: []Step{StepBudgets}}
	st := Resolve(july, full(), reviewNow)
	if st.Index != 0 || st.Current != StepRecap {
		t.Fatalf("state = %+v, want a fresh start", st)
	}
	if st.Done || st.Dismissed {
		t.Fatalf("last month's state carried over: %+v", st)
	}
}

func TestTheIndexIsClampedWhenStepsDisappearBetweenSittings(t *testing.T) {
	// A budget trued up on Tuesday removes its own step by Thursday; resuming to
	// the stored index would run off the end of the list.
	p := Progress{Month: MonthKey(reviewNow), StepIndex: 4}
	st := Resolve(p, Availability{}, reviewNow) // only recap + close remain
	if st.Index != 1 || st.Current != StepClose {
		t.Fatalf("state = %+v, want the last remaining step", st)
	}
}

func TestAdvanceWalksToTheEndAndStops(t *testing.T) {
	a := full()
	p := Progress{}
	for i := 0; i < len(VisibleSteps(a)); i++ {
		p = Advance(p, a, reviewNow, false)
	}
	if !p.Done {
		t.Fatalf("the review did not finish: %+v", p)
	}
	if ShouldOffer(p, a, reviewNow) {
		t.Fatal("a finished review was offered again the same month")
	}
}

func TestSkippingIsRecordedAsADecision(t *testing.T) {
	// The point is to reach the end having decided about each step, and "not this
	// month" is a decision — so the close can say what was left rather than
	// implying everything was handled.
	a := full()
	p := Advance(Progress{}, a, reviewNow, false) // recap → findings
	p = Advance(p, a, reviewNow, true)            // skip findings
	if len(p.Skipped) != 1 || p.Skipped[0] != StepFindings {
		t.Fatalf("skipped = %v", p.Skipped)
	}
	// Skipping the same step twice records it once.
	p.StepIndex = 1
	p = Advance(p, a, reviewNow, true)
	if len(p.Skipped) != 1 {
		t.Fatalf("skipped = %v, want no duplicate", p.Skipped)
	}
}

func TestANewMonthStartsWithACleanSkipList(t *testing.T) {
	p := Progress{Month: "2026-07", Skipped: []Step{StepBudgets, StepGoals}, Done: true}
	next := Advance(p, full(), reviewNow, false)
	if len(next.Skipped) != 0 {
		t.Fatalf("skipped = %v, want July's skips left behind", next.Skipped)
	}
	if next.Done {
		t.Fatal("August started already finished")
	}
}

func TestTheCloseSaysWhatActuallyHappened(t *testing.T) {
	a := full()
	all := VisibleSteps(a)

	if got := CloseSummary(Progress{}, a, reviewNow); !strings.Contains(got, "everything") {
		t.Fatalf("nothing skipped = %q", got)
	}
	partial := Progress{Skipped: []Step{StepBudgets}}
	if got := CloseSummary(partial, a, reviewNow); !strings.Contains(got, "left 1") {
		t.Fatalf("one skipped = %q", got)
	}
	everything := Progress{Skipped: all}
	got := CloseSummary(everything, a, reviewNow)
	if !strings.Contains(got, "that's a decision too") {
		t.Fatalf("all skipped = %q — skipping is not a failure state", got)
	}
}

func TestAtLastKnowsWhenTheNextButtonShouldFinish(t *testing.T) {
	a := full()
	steps := VisibleSteps(a)
	st := Resolve(Progress{Month: MonthKey(reviewNow), StepIndex: len(steps) - 1}, a, reviewNow)
	if !st.AtLast() {
		t.Fatalf("state = %+v, want the last step", st)
	}
	first := Resolve(Progress{Month: MonthKey(reviewNow)}, a, reviewNow)
	if first.AtLast() {
		t.Fatal("the first of several steps reported as the last")
	}
}

func TestMonthKeyIsStableAcrossTimeZones(t *testing.T) {
	tokyo := time.FixedZone("JST", 9*3600)
	late := time.Date(2026, time.September, 1, 2, 0, 0, 0, tokyo) // 31 August UTC
	if got := MonthKey(late); got != "2026-08" {
		t.Fatalf("MonthKey = %q, want the UTC month so a synced dataset agrees with itself", got)
	}
}
